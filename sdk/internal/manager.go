package internal

import (
	"context"
	"fmt"
	"reflect"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// PluginManager defines the contract for managing WASM plugins and host functions
type PluginManager interface {
	// Call executes a function in any module (WASM or host)
	Call(moduleName, functionName string, input []byte) (int32, []byte, error)
	// Close releases all resources
	Close() error
}

type manager struct {
	runtime          wazero.Runtime
	wasmModules      map[string]api.Module
	hostModules      map[string]api.Module
	config           Config
	envModuleName    string
	memoryOffsets    map[string]uint32
	currentInputPtr  uint32
	currentInputLen  uint32
	currentOutputPtr uint32
	currentOutputLen uint32
}

type Config struct {
	EnableWASI    bool
	EnvModuleName string
}

type Module struct {
	Name     string
	WasmData []byte
}

type HostFunction struct {
	ModuleName   string
	FunctionName string
	Handler      any
}

func NewManager(ctx context.Context, config Config, wasmModules []Module, hostFunctions []HostFunction) (PluginManager, error) {
	r := wazero.NewRuntime(ctx)

	if config.EnableWASI {
		wasi_snapshot_preview1.MustInstantiate(ctx, r)
	}

	envName := config.EnvModuleName
	if envName == "" {
		envName = "env"
	}
	mgr := &manager{
		runtime:       r,
		wasmModules:   make(map[string]api.Module),
		hostModules:   make(map[string]api.Module),
		config:        config,
		envModuleName: envName,
		memoryOffsets: make(map[string]uint32),
	}

	// Setup host functions first
	if err := mgr.setupHostFunctions(ctx, hostFunctions); err != nil {
		mgr.runtime.Close(ctx)
		return nil, fmt.Errorf("failed to setup host functions: %w", err)
	}

	// Load all WASM modules
	for _, module := range wasmModules {
		if err := mgr.loadWasmModule(ctx, module); err != nil {
			mgr.runtime.Close(ctx)
			return nil, fmt.Errorf("failed to load module %s: %w", module.Name, err)
		}
	}

	return mgr, nil
}

func (m *manager) setupHostFunctions(ctx context.Context, hostFunctions []HostFunction) error {
	// Group host functions by module name
	hostModules := make(map[string][]HostFunction)
	for _, fn := range hostFunctions {
		hostModules[fn.ModuleName] = append(hostModules[fn.ModuleName], fn)
	}

	// Add default env module if not provided
	if _, exists := hostModules[m.envModuleName]; !exists {
		hostModules[m.envModuleName] = []HostFunction{
			{ModuleName: m.envModuleName, FunctionName: "alloc", Handler: nil},
			{ModuleName: m.envModuleName, FunctionName: "free", Handler: nil},
			{ModuleName: m.envModuleName, FunctionName: "input_ptr", Handler: nil},
			{ModuleName: m.envModuleName, FunctionName: "input_len", Handler: nil},
			{ModuleName: m.envModuleName, FunctionName: "set_output", Handler: nil},
		}
	}

	// Create host modules for each group
	for moduleName, functions := range hostModules {
		if err := m.createHostModule(ctx, moduleName, functions); err != nil {
			return fmt.Errorf("failed to create host module %s: %w", moduleName, err)
		}
	}

	return nil
}

func (m *manager) createHostModule(ctx context.Context, moduleName string, functions []HostFunction) error {
	builder := m.runtime.NewHostModuleBuilder(moduleName)

	if moduleName == m.envModuleName {
		// Hardcoded "env" functions
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.allocFunc), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI32}).
			Export("alloc")
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.freeFunc), []api.ValueType{api.ValueTypeI32}, nil).
			Export("free")
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.inputPtrFunc), nil, []api.ValueType{api.ValueTypeI32}).
			Export("input_ptr")
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.inputLenFunc), nil, []api.ValueType{api.ValueTypeI32}).
			Export("input_len")
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.setOutputFunc), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil).
			Export("set_output")
	} else {
		for _, fn := range functions {
			if err := m.registerHostFunction(builder, fn); err != nil {
				return fmt.Errorf("failed to register function %s: %w", fn.FunctionName, err)
			}
		}
	}

	mod, err := builder.Instantiate(ctx)
	if err != nil {
		return fmt.Errorf("failed to instantiate host module: %w", err)
	}

	m.hostModules[moduleName] = mod
	return nil
}

func (m *manager) registerHostFunction(builder wazero.HostModuleBuilder, fn HostFunction) error {
	// Use reflection to determine function signature and register appropriately
	handlerValue := reflect.ValueOf(fn.Handler)
	handlerType := handlerValue.Type()

	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function")
	}

	// For now, support simple function signatures
	// TODO: Add support for more complex signatures
	numIn := handlerType.NumIn()
	numOut := handlerType.NumOut()

	// Convert to wazero types
	var paramTypes []api.ValueType
	var resultTypes []api.ValueType

	for i := range numIn {
		paramTypes = append(paramTypes, goTypeToWazeroType(handlerType.In(i)))
	}

	for i := range numOut {
		resultTypes = append(resultTypes, goTypeToWazeroType(handlerType.Out(i)))
	}

	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			// Convert stack arguments to Go values
			args := make([]reflect.Value, numIn)
			for i := range numIn {
				args[i] = reflect.ValueOf(convertWazeroToGo(stack[i], handlerType.In(i)))
			}

			// Call the handler function
			results := handlerValue.Call(args)

			// Convert results back to stack
			for i, result := range results {
				if i < len(stack) {
					stack[i] = convertGoToWazero(result.Interface())
				}
			}
		}), paramTypes, resultTypes).
		WithParameterNames(getParameterNames(handlerType)...).
		Export(fn.FunctionName)

	return nil
}

func (m *manager) loadWasmModule(ctx context.Context, module Module) error {
	config := wazero.NewModuleConfig().WithName(module.Name)
	mod, err := m.runtime.InstantiateWithConfig(ctx, module.WasmData, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate module %s: %w", module.Name, err)
	}
	m.wasmModules[module.Name] = mod
	return nil
}

func (m *manager) Call(moduleName, functionName string, input []byte) (int32, []byte, error) {
	return m.CallWithContext(context.Background(), moduleName, functionName, input)
}

func (m *manager) CallWithContext(ctx context.Context, moduleName, functionName string, input []byte) (int32, []byte, error) {
	// Try WASM modules first
	if mod, exists := m.wasmModules[moduleName]; exists {
		return m.callWasmFunction(ctx, mod, functionName, input)
	}

	// Try host modules
	if mod, exists := m.hostModules[moduleName]; exists {
		return m.callHostFunction(ctx, mod, functionName, input)
	}

	return 0, nil, fmt.Errorf("module %s not found", moduleName)
}

func (m *manager) callWasmFunction(ctx context.Context, mod api.Module, functionName string, input []byte) (int32, []byte, error) {
	// Write input to memory
	inputPtr := m.allocate(mod, uint32(len(input)))
	memory := mod.Memory()
	if !memory.Write(inputPtr, input) {
		return 0, nil, fmt.Errorf("failed to write input to memory")
	}
	m.currentInputPtr = inputPtr
	m.currentInputLen = uint32(len(input))

	// Reset output
	m.currentOutputPtr = 0
	m.currentOutputLen = 0

	fn := mod.ExportedFunction(functionName)
	if fn == nil {
		return 0, nil, fmt.Errorf("function %s not found in module", functionName)
	}

	results, err := fn.Call(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to call function %s: %w", functionName, err)
	}

	returnValue := int32(0)
	if len(results) > 0 {
		returnValue = int32(results[0])
	}

	// Read output
	if m.currentOutputPtr == 0 {
		return returnValue, []byte{}, nil
	}
	output, ok := memory.Read(m.currentOutputPtr, m.currentOutputLen)
	if !ok {
		return 0, nil, fmt.Errorf("failed to read output from memory")
	}
	return returnValue, output, nil
}

func (m *manager) callHostFunction(ctx context.Context, mod api.Module, functionName string, input []byte) (int32, []byte, error) {
	fn := mod.ExportedFunction(functionName)
	if fn == nil {
		return 0, nil, fmt.Errorf("function %s not found in module", functionName)
	}

	// TODO: Implement proper input/output handling for host functions
	_, err := fn.Call(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to call function %s: %w", functionName, err)
	}

	// TODO: Return actual output data
	return 0, []byte{}, nil
}

func (m *manager) Close() error {
	// Close all WASM modules
	for _, mod := range m.wasmModules {
		mod.Close(context.Background())
	}

	// Close all host modules
	for _, mod := range m.hostModules {
		mod.Close(context.Background())
	}

	return m.runtime.Close(context.Background())
}

// Helper functions
func goTypeToWazeroType(goType reflect.Type) api.ValueType {
	switch goType.Kind() {
	case reflect.Uint32:
		return api.ValueTypeI32
	case reflect.Uint64:
		return api.ValueTypeI64
	case reflect.Float32:
		return api.ValueTypeF32
	case reflect.Float64:
		return api.ValueTypeF64
	default:
		// Default to I32 for unsupported types
		return api.ValueTypeI32
	}
}

func getParameterNames(handlerType reflect.Type) []string {
	numIn := handlerType.NumIn()
	names := make([]string, numIn)
	for i := range numIn {
		names[i] = fmt.Sprintf("param%d", i)
	}
	return names
}

func convertWazeroToGo(value uint64, goType reflect.Type) any {
	switch goType.Kind() {
	case reflect.Uint32:
		return uint32(value)
	case reflect.Uint64:
		return value
	case reflect.Float32:
		return float32(value) // This is a simplification
	case reflect.Float64:
		return float64(value) // This is a simplification
	default:
		return uint32(value) // Default to uint32
	}
}

func convertGoToWazero(value any) uint64 {
	switch v := value.(type) {
	case uint32:
		return uint64(v)
	case uint64:
		return v
	case float32:
		return uint64(v) // This is a simplification
	case float64:
		return uint64(v) // This is a simplification
	default:
		return 0
	}
}

func (m *manager) allocFunc(ctx context.Context, mod api.Module, stack []uint64) {
	size := uint32(stack[0])
	ptr := m.allocate(mod, size)
	stack[0] = uint64(ptr)
}

func (m *manager) freeFunc(ctx context.Context, mod api.Module, stack []uint64) {
	// Do nothing for now
}

func (m *manager) inputPtrFunc(ctx context.Context, mod api.Module, stack []uint64) {
	stack[0] = uint64(m.currentInputPtr)
}

func (m *manager) inputLenFunc(ctx context.Context, mod api.Module, stack []uint64) {
	stack[0] = uint64(m.currentInputLen)
}

func (m *manager) setOutputFunc(ctx context.Context, mod api.Module, stack []uint64) {
	m.currentOutputPtr = uint32(stack[0])
	m.currentOutputLen = uint32(stack[1])
}

func (m *manager) allocate(mod api.Module, size uint32) uint32 {
	name := mod.Name()
	offset := m.memoryOffsets[name]
	m.memoryOffsets[name] = offset + size
	return offset
}

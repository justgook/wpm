package internal

import (
	"context"
	"fmt"
	"io/fs"
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

type callContext struct {
	moduleName string
	inputPtr   uint32
	inputLen   uint32
	outputPtr  uint32
	outputLen  uint32
}

type manager struct {
	runtime          wazero.Runtime
	wasmModules      map[string]api.Module
	hostModules      map[string]api.Module
	hostFunctionDefs map[string]map[string]HostFunction // moduleName -> functionName -> HostFunction
	config           Config
	envModuleName    string
	memoryOffsets    map[string]uint32
	currentInputPtr  uint32
	currentInputLen  uint32
	currentOutputPtr uint32
	currentOutputLen uint32
	// Plugin-to-plugin call support
	callStack         []*callContext
	lastCallReturn    int32
	lastCallOutputPtr uint32
	lastCallOutputLen uint32
}

type Config struct {
	EnableWASI    bool
	EnvModuleName string
	MaxCallDepth  int // Maximum depth for nested plugin calls (default: 10)
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

// ByteHandler is a special handler type for host functions that work with byte slices
type ByteHandler func(input []byte) (int32, []byte)

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
		runtime:          r,
		wasmModules:      make(map[string]api.Module),
		hostModules:      make(map[string]api.Module),
		hostFunctionDefs: make(map[string]map[string]HostFunction),
		config:           config,
		envModuleName:    envName,
		memoryOffsets:    make(map[string]uint32),
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

		// Store function definitions for direct invocation
		if m.hostFunctionDefs[fn.ModuleName] == nil {
			m.hostFunctionDefs[fn.ModuleName] = make(map[string]HostFunction)
		}
		m.hostFunctionDefs[fn.ModuleName][fn.FunctionName] = fn
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
		// Plugin-to-plugin call functions
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.pluginCallFunc), []api.ValueType{
				api.ValueTypeI32, api.ValueTypeI32, // module name ptr, len
				api.ValueTypeI32, api.ValueTypeI32, // function name ptr, len
				api.ValueTypeI32, api.ValueTypeI32, // input ptr, len
			}, []api.ValueType{api.ValueTypeI32}). // status code
			Export("plugin_call")
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.pluginCallReturnFunc), nil, []api.ValueType{api.ValueTypeI32}).
			Export("plugin_call_return")
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.pluginCallOutputPtrFunc), nil, []api.ValueType{api.ValueTypeI32}).
			Export("plugin_call_output_ptr")
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(m.pluginCallOutputLenFunc), nil, []api.ValueType{api.ValueTypeI32}).
			Export("plugin_call_output_len")
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
	// Check if this is a ByteHandler (special case for byte-based functions)
	if byteHandler, ok := fn.Handler.(ByteHandler); ok {
		// ByteHandler functions work like plugin functions:
		// - They read input via env.input_ptr/input_len
		// - They write output via env.set_output
		// - They return a status code (uint32)
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				// This is a host function in a host module, but we need to access
				// the calling WASM module's memory for input/output
				// We use the manager's current input/output state
				var input []byte
				if m.currentInputLen > 0 {
					// Find the calling module (last in call stack or use context)
					callingMod := mod
					if len(m.callStack) > 0 {
						// Get the module from the call stack
						lastCtx := m.callStack[len(m.callStack)-1]
						if wasmMod, exists := m.wasmModules[lastCtx.moduleName]; exists {
							callingMod = wasmMod
						}
					}

					memory := callingMod.Memory()
					if memory != nil {
						inputBytes, ok := memory.Read(m.currentInputPtr, m.currentInputLen)
						if ok {
							input = inputBytes
						}
					}
				}

				// Call the handler
				returnCode, output := byteHandler(input)

				// Write output back to calling module's memory
				if len(output) > 0 {
					callingMod := mod
					if len(m.callStack) > 0 {
						lastCtx := m.callStack[len(m.callStack)-1]
						if wasmMod, exists := m.wasmModules[lastCtx.moduleName]; exists {
							callingMod = wasmMod
						}
					}

					memory := callingMod.Memory()
					if memory != nil {
						outputPtr := m.allocate(callingMod, uint32(len(output)))
						if memory.Write(outputPtr, output) {
							m.currentOutputPtr = outputPtr
							m.currentOutputLen = uint32(len(output))
						}
					}
				} else {
					m.currentOutputPtr = 0
					m.currentOutputLen = 0
				}

				// Return the status code
				stack[0] = uint64(uint32(returnCode))
			}), nil, []api.ValueType{api.ValueTypeI32}).
			Export(fn.FunctionName)

		return nil
	}

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

	// If WASI is enabled, configure it with an empty filesystem
	// This prevents nil pointer errors when Go's runtime tries to access filesystem
	if m.config.EnableWASI {
		config = config.WithFS(emptyFS{})
	}

	mod, err := m.runtime.InstantiateWithConfig(ctx, module.WasmData, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate module %s: %w", module.Name, err)
	}

	// Call _initialize if it exists (required for TinyGo modules)
	if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			return fmt.Errorf("failed to initialize module %s: %w", module.Name, err)
		}
	}

	m.wasmModules[module.Name] = mod
	return nil
}

// emptyFS is a minimal fs.FS implementation that returns "file not found" for everything
type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
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

	// Handle both functions with and without return values
	// Functions using //go:wasmexport typically have no return value
	// Functions using //export may return a value
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
	// Get the host function definition
	moduleName := mod.Name()
	moduleFuncs, exists := m.hostFunctionDefs[moduleName]
	if !exists {
		return 0, nil, fmt.Errorf("host module %s not found", moduleName)
	}

	fn, exists := moduleFuncs[functionName]
	if !exists {
		return 0, nil, fmt.Errorf("function %s not found in host module %s", functionName, moduleName)
	}

	// Check if this is a ByteHandler
	if byteHandler, ok := fn.Handler.(ByteHandler); ok {
		// Directly call the byte handler
		returnCode, output := byteHandler(input)
		return returnCode, output, nil
	}

	// For primitive type handlers, we need to use reflection
	// This is more complex and requires converting input bytes to appropriate types
	// For now, return an error for primitive handlers when called via manager.Call
	// They should be called from plugins via direct WASM imports
	return 0, nil, fmt.Errorf("primitive type host functions (like %s.%s) should be called from plugins via direct WASM imports, not via manager.Call", moduleName, functionName)
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

// Plugin-to-plugin call functions

func (m *manager) pluginCallFunc(ctx context.Context, mod api.Module, stack []uint64) {
	// 1. Read parameters from stack
	modulePtr := uint32(stack[0])
	moduleLen := uint32(stack[1])
	funcPtr := uint32(stack[2])
	funcLen := uint32(stack[3])
	inputPtr := uint32(stack[4])
	inputLen := uint32(stack[5])

	// 2. Read strings from caller's memory
	memory := mod.Memory()
	moduleNameBytes, ok := memory.Read(modulePtr, moduleLen)
	if !ok {
		stack[0] = 1 // error: failed to read module name
		return
	}
	funcNameBytes, ok := memory.Read(funcPtr, funcLen)
	if !ok {
		stack[0] = 2 // error: failed to read function name
		return
	}

	moduleName := string(moduleNameBytes)
	funcName := string(funcNameBytes)

	// Read input (may be empty)
	var inputBytes []byte
	if inputLen > 0 {
		inputBytes, ok = memory.Read(inputPtr, inputLen)
		if !ok {
			stack[0] = 3 // error: failed to read input
			return
		}
	}

	// 3. Check call depth
	maxDepth := m.config.MaxCallDepth
	if maxDepth == 0 {
		maxDepth = 10 // default
	}
	if len(m.callStack) >= maxDepth {
		stack[0] = 4 // error: max call depth exceeded
		return
	}

	// 4. Save current context
	currentCtx := &callContext{
		moduleName: mod.Name(),
		inputPtr:   m.currentInputPtr,
		inputLen:   m.currentInputLen,
		outputPtr:  m.currentOutputPtr,
		outputLen:  m.currentOutputLen,
	}
	m.callStack = append(m.callStack, currentCtx)

	// 5. Make the actual call
	returnValue, output, err := m.CallWithContext(ctx, moduleName, funcName, inputBytes)

	// 6. Restore context
	m.callStack = m.callStack[:len(m.callStack)-1]
	m.currentInputPtr = currentCtx.inputPtr
	m.currentInputLen = currentCtx.inputLen
	m.currentOutputPtr = currentCtx.outputPtr
	m.currentOutputLen = currentCtx.outputLen

	if err != nil {
		stack[0] = 5 // error: call failed
		return
	}

	// 7. Store call results in manager state for later retrieval
	m.lastCallReturn = returnValue

	// Allocate memory in caller's space for output
	if len(output) > 0 {
		m.lastCallOutputPtr = m.allocate(mod, uint32(len(output)))
		m.lastCallOutputLen = uint32(len(output))
		if !memory.Write(m.lastCallOutputPtr, output) {
			stack[0] = 6 // error: failed to write output
			return
		}
	} else {
		m.lastCallOutputPtr = 0
		m.lastCallOutputLen = 0
	}

	stack[0] = 0 // success
}

func (m *manager) pluginCallReturnFunc(ctx context.Context, mod api.Module, stack []uint64) {
	stack[0] = uint64(uint32(m.lastCallReturn))
}

func (m *manager) pluginCallOutputPtrFunc(ctx context.Context, mod api.Module, stack []uint64) {
	stack[0] = uint64(m.lastCallOutputPtr)
}

func (m *manager) pluginCallOutputLenFunc(ctx context.Context, mod api.Module, stack []uint64) {
	stack[0] = uint64(m.lastCallOutputLen)
}

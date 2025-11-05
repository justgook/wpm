package sdk

import (
	"context"

	"github.com/justgook/wpm/sdk/internal"
)

// PluginManager defines the contract for managing WASM plugins and host functions
type PluginManager = internal.PluginManager

// Config holds plugin manager configuration
type Config struct {
	EnableWASI    bool
	EnvModuleName string
	MaxCallDepth  int // Maximum depth for nested plugin calls (default: 10)
}

// Module represents a WASM plugin module
type Module struct {
	Name     string
	WasmData []byte
}

// HostFunction represents any Go function that can be called from WASM
type HostFunction struct {
	Module   string // e.g., "memory", "game", "io"
	Function string // e.g., "alloc", "get_player_position"
	Handler  any    // any function signature wazero supports
}

// ByteHandler is a special handler type for host functions that work with byte slices
// This signature mirrors the plugin Call interface: (input []byte) -> (returnCode int32, output []byte)
type ByteHandler func(input []byte) (int32, []byte)

// New creates a plugin manager with WASM modules and host functions
func New(ctx context.Context, config Config, wasmModules []Module, hostFunctions []HostFunction) (PluginManager, error) {
	internalModules := make([]internal.Module, len(wasmModules))
	for i, m := range wasmModules {
		internalModules[i] = internal.Module{
			Name:     m.Name,
			WasmData: m.WasmData,
		}
	}

	internalHostFunctions := make([]internal.HostFunction, len(hostFunctions))
	for i, f := range hostFunctions {
		// Convert ByteHandler to internal.ByteHandler if needed
		var handler any = f.Handler
		if bh, ok := f.Handler.(ByteHandler); ok {
			handler = internal.ByteHandler(bh)
		}

		internalHostFunctions[i] = internal.HostFunction{
			ModuleName:   f.Module,
			FunctionName: f.Function,
			Handler:      handler,
		}
	}

	internalConfig := internal.Config{
		EnableWASI:    config.EnableWASI,
		EnvModuleName: config.EnvModuleName,
		MaxCallDepth:  config.MaxCallDepth,
	}

	return internal.NewManager(ctx, internalConfig, internalModules, internalHostFunctions)
}

package sdk

import (
	"context"

	"github.com/justgook/wasm-plugin/sdk/internal"
)

// PluginManager defines the contract for managing WASM plugins and host functions
type PluginManager = internal.PluginManager

// Config holds plugin manager configuration
type Config struct {
	EnableWASI    bool
	EnvModuleName string
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
		internalHostFunctions[i] = internal.HostFunction{
			ModuleName:   f.Module,
			FunctionName: f.Function,
			Handler:      f.Handler,
		}
	}

	internalConfig := internal.Config{
		EnableWASI:    config.EnableWASI,
		EnvModuleName: config.EnvModuleName,
	}

	return internal.NewManager(ctx, internalConfig, internalModules, internalHostFunctions)
}

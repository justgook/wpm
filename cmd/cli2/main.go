package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/justgook/wasm-plugin/sdk"
)

func main() {
	ctx := context.Background()

	// Load WASM module
	wasmBytes, err := os.ReadFile("build.nosync/greet.wasm")
	if err != nil {
		log.Fatal(err)
	}

	// Define host functions (equivalent to the old manual setup)
	hostFunctions := []sdk.HostFunction{
		{
			Module:   "env",
			Function: "alloc",
			Handler: func(size uint64) uint32 {
				// Simple memory allocation (placeholder)
				return uint32(size) // Just return the size as a fake pointer
			},
		},
		{
			Module:   "env",
			Function: "free",
			Handler: func(ptr uint32) {
				// Simple memory free (placeholder)
				_ = ptr
			},
		},
		{
			Module:   "env",
			Function: "input_ptr",
			Handler: func() uint32 {
				// Return pointer to input data (placeholder)
				return 0
			},
		},
		{
			Module:   "env",
			Function: "input_len",
			Handler: func() uint32 {
				// Return length of input data (placeholder)
				return 5 // "Alice" length
			},
		},
		{
			Module:   "env",
			Function: "set_output",
			Handler: func(ptr uint32, len uint32) {
				// Set output buffer info (placeholder)
				_ = ptr
				_ = len
			},
		},
	}

	// Create plugin manager with WASM modules and host functions
	manager, err := sdk.New(ctx, sdk.Config{EnableWASI: true},
		[]sdk.Module{
			{Name: "greet", WasmData: wasmBytes},
		},
		hostFunctions,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()

	// Call the greet function
	returnValue, result, err := manager.Call("greet", "greet", []byte("Alice"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Plugin return value: %d\n", returnValue)
	fmt.Printf("Plugin output: %s\n", string(result))
}

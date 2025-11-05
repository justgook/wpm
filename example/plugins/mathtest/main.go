package main

import (
	"fmt"

	"github.com/justgook/wpm/pdk"
)

// Import primitive host functions via direct WASM imports
//
//go:wasmimport math add
func mathAdd(a, b uint32) uint32

//go:wasmimport math multiply
func mathMultiply(a, b uint32) uint32

//export calculate
func Calculate() uint32 {
	// Call primitive host functions directly
	sum := mathAdd(5, 3)
	product := mathMultiply(4, 7)

	// Also demonstrate calling byte-based host functions
	_, upperResult, _ := pdk.Call("native_host", "uppercase", []byte("math result"))

	// Format output
	output := fmt.Sprintf("Sum: %d, Product: %d, Upper: %s", sum, product, string(upperResult))
	pdk.Output([]byte(output))
	return 0
}

func main() {}

package pdk

import (
	"fmt"
	"unsafe"
)

//go:wasmimport env alloc
func alloc(size uint64) uint32

//go:wasmimport env free
func free(ptr uint32)

//go:wasmimport env input_ptr
func inputPtr() uint32

//go:wasmimport env input_len
func inputLen() uint32

//go:wasmimport env set_output
func setOutput(ptr uint32, len uint32)

//go:wasmimport env plugin_call
func pluginCall(modulePtr, moduleLen, funcPtr, funcLen, inputPtr, inputLen uint32) uint32

//go:wasmimport env plugin_call_return
func pluginCallReturn() int32

//go:wasmimport env plugin_call_output_ptr
func pluginCallOutputPtr() uint32

//go:wasmimport env plugin_call_output_len
func pluginCallOutputLen() uint32

// Core ABI functions - direct re-exports
func Alloc(size uint64) uint32 {
	return alloc(size)
}

func Free(ptr uint32) {
	free(ptr)
}

func InputPtr() uint32 {
	return inputPtr()
}

func InputLen() uint32 {
	return inputLen()
}

func SetOutput(ptr uint32, len uint32) {
	setOutput(ptr, len)
}

// Convenience sugar - returns both ptr and len at once
func InputInfo() (ptr uint32, len uint32) {
	return inputPtr(), inputLen()
}

// High-level helpers - handle unsafe pointer conversion internally
func Input() []byte {
	ptr, len := InputInfo()
	return (*[1 << 20]byte)(unsafe.Pointer(uintptr(ptr)))[:len:len]
}

func Output(data []byte) {
	outPtr := Alloc(uint64(len(data)))
	outBuf := (*[1 << 20]byte)(unsafe.Pointer(uintptr(outPtr)))[:len(data):len(data)]
	copy(outBuf, data)
	SetOutput(outPtr, uint32(len(data)))
}

// Plugin-to-plugin communication

// Call invokes a function in another module (plugin or host)
// Returns: (returnValue int32, output []byte, error)
func Call(moduleName, functionName string, input []byte) (int32, []byte, error) {
	// 1. Allocate and write module name
	modulePtr := Alloc(uint64(len(moduleName)))
	moduleBuf := ptrToBytes(modulePtr, uint32(len(moduleName)))
	copy(moduleBuf, moduleName)

	// 2. Allocate and write function name
	funcPtr := Alloc(uint64(len(functionName)))
	funcBuf := ptrToBytes(funcPtr, uint32(len(functionName)))
	copy(funcBuf, functionName)

	// 3. Allocate and write input
	var inputPtr uint32
	if len(input) > 0 {
		inputPtr = Alloc(uint64(len(input)))
		inputBuf := ptrToBytes(inputPtr, uint32(len(input)))
		copy(inputBuf, input)
	}

	// 4. Make the call
	result := pluginCall(
		modulePtr, uint32(len(moduleName)),
		funcPtr, uint32(len(functionName)),
		inputPtr, uint32(len(input)),
	)

	// 5. Free allocated memory
	Free(modulePtr)
	Free(funcPtr)
	if inputPtr != 0 {
		Free(inputPtr)
	}

	// 6. Check for errors (result is status code)
	if result != 0 {
		return 0, nil, fmt.Errorf("plugin call failed with code %d", result)
	}

	// 7. Get return value and output
	returnValue := pluginCallReturn()
	outputPtr := pluginCallOutputPtr()
	outputLen := pluginCallOutputLen()

	// 8. Copy output
	var output []byte
	if outputLen > 0 {
		output = make([]byte, outputLen)
		outputBuf := ptrToBytes(outputPtr, outputLen)
		copy(output, outputBuf)
	}

	return returnValue, output, nil
}

// CallPlugin is a convenience wrapper for calling other plugins
func CallPlugin(pluginName, functionName string, input []byte) (int32, []byte, error) {
	return Call(pluginName, functionName, input)
}

// CallHost is a convenience wrapper for calling host functions
func CallHost(functionName string, input []byte) (int32, []byte, error) {
	return Call("host", functionName, input)
}

// Helper function to convert pointer to byte slice
func ptrToBytes(ptr uint32, length uint32) []byte {
	return (*[1 << 20]byte)(unsafe.Pointer(uintptr(ptr)))[:length:length]
}

package pdk

import "unsafe"

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

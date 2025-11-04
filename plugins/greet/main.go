package main

import (
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

//go:wasmimport random next
func nextRand() float32

//export greet
func Greet() uint32 {
	nextRand()
	inPtr := inputPtr()
	inLen := inputLen()
	inData := (*[1 << 20]byte)(unsafe.Pointer(uintptr(inPtr)))[:inLen:inLen]
	name := string(inData)

	// Create greeting
	greeting := "Hello " + name

	// Allocate space for output
	outPtr := alloc(uint64(len(greeting)))
	outBuf := (*[1 << 20]byte)(unsafe.Pointer(uintptr(outPtr)))[:len(greeting):len(greeting)]
	copy(outBuf, greeting)

	// Register output buffer
	setOutput(outPtr, uint32(len(greeting)))
	return 0
}

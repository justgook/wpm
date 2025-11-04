package main

import (
	"github.com/justgook/wasm-plugin/pdk"
)

//go:wasmimport random next
func nextRand() float32

//export greet
func Greet() uint32 {
	input := pdk.Input()
	name := string(input)

	// List of random greetings
	greetings := []string{"Hello", "Hi", "Hey there", "Greetings", "Salutations", "Welcome"}
	// Use random to pick a greeting
	randIndex := int(nextRand() * float32(len(greetings)))
	selectedGreeting := greetings[randIndex]

	// Create personalized greeting
	greeting := selectedGreeting + ", " + name + "!"
	pdk.Output([]byte(greeting))
	return 0
}

package main

import (
	"github.com/justgook/wpm/pdk"
)

//go:wasmimport random next
func nextRand() float32

//export greet
func Greet() uint32 {
	input := pdk.Input()
	name := string(input)

	// List of random greetings
	greetings := []string{"Hello", "Hi", "Hey there", "Greetings", "Salutations", "Welcome"}
	// Use random to pick a greeting (using direct WASM import for performance)
	randIndex := int(nextRand() * float32(len(greetings)))
	selectedGreeting := greetings[randIndex]

	// Create personalized greeting
	greeting := selectedGreeting + ", " + name + "!"

	// Call logger plugin using pdk.Call for data exchange
	logMessage := "Generated greeting for " + name
	_, logOutput, err := pdk.Call("logger", "log", []byte(logMessage))
	if err != nil {
		// If logging fails, add error to output
		greeting = greeting + " (logging failed: " + err.Error() + ")"
	} else {
		// Add log confirmation to output
		greeting = greeting + " | Logger says: " + string(logOutput)
	}

	pdk.Output([]byte(greeting))
	return 0
}

func main() {}

package main

import (
	"github.com/justgook/wpm/pdk"
)

//export log
func Log() uint32 {
	input := pdk.Input()
	message := string(input)

	// Format the log message
	output := "[LOG] " + message

	// Call host to actually print to console
	_, _, err := pdk.Call("host", "print", []byte(output))
	if err != nil {
		// If host call fails, just return the formatted message
		pdk.Output([]byte(output + " (host print failed)"))
		return 1
	}

	// Return the formatted log message
	pdk.Output([]byte(output))
	return 0
}

func main() {}

package main

import (
	"fmt"
	"time"

	"github.com/justgook/wpm/pdk"
)

//export print
func Print() uint32 {
	input := pdk.Input()
	message := string(input)

	// Print to stdout (this runs in the host Go runtime)
	fmt.Println("[HOST]", message)

	// Return confirmation
	pdk.Output([]byte("printed"))
	return 0
}

//export get_timestamp
func GetTimestamp() uint32 {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	pdk.Output([]byte(timestamp))
	return 0
}

func main() {}

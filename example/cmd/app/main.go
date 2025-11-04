package main

import (
	"context"
	"fmt"
	"os"

	"github.com/justgook/wasm-plugin/sdk"
)

func main() {
	ctx := context.Background()

	manager := Must(sdk.New(ctx, sdk.Config{
		EnableWASI: true,
	}, []sdk.Module{
		{Name: "random", WasmData: Must(os.ReadFile("random.wasm"))},
		{Name: "greet", WasmData: Must(os.ReadFile("greet.wasm"))},
	}, []sdk.HostFunction{}))
	defer manager.Close()

	// Test greet function
	returnValue, result := Must2(manager.Call("greet", "greet", []byte("World")))
	fmt.Printf("Greet - return value: %d, output: %s\n", returnValue, string(result))
}

func Must0(err error) {
	if err != nil {
		panic(err)
	}
}

func Must[T any](x T, err error) T {
	if err != nil {
		panic(err)
	}

	return x
}

func Must2[T1 any, T2 any](obj1 T1, obj2 T2, err error) (T1, T2) {
	if err != nil {
		panic(err)
	}

	return obj1, obj2
}

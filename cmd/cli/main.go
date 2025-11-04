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
		{Name: "random", WasmData: Must(os.ReadFile("build.nosync/random.wasm"))},
		{Name: "greet", WasmData: Must(os.ReadFile("build.nosync/greet.wasm"))},
	}, []sdk.HostFunction{}))
	defer manager.Close()

	returnValue, result := Must2(manager.Call("greet", "greet", []byte("Kazys")))
	fmt.Printf("Plugin return value: %d\n", returnValue)
	fmt.Printf("Plugin output: %s\n", string(result))
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

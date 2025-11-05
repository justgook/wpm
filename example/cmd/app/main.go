package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/justgook/wpm/sdk"
)

func main() {
	ctx := context.Background()

	manager := Must(sdk.New(ctx, sdk.Config{
		EnableWASI:   true,
		MaxCallDepth: 10,
	}, []sdk.Module{
		{Name: "host", WasmData: Must(os.ReadFile("host.wasm"))}, // Host functions as a plugin (for comparison)
		{Name: "random", WasmData: Must(os.ReadFile("random.wasm"))},
		{Name: "logger", WasmData: Must(os.ReadFile("logger.wasm"))},
		{Name: "greet", WasmData: Must(os.ReadFile("greet.wasm"))},
		{Name: "mathtest", WasmData: Must(os.ReadFile("mathtest.wasm"))}, // Demonstrates calling primitive host functions
	}, []sdk.HostFunction{
		// === Primitive Type Host Functions (simple, zero-copy) ===
		{
			Module:   "math",
			Function: "add",
			Handler: func(a, b uint32) uint32 {
				return a + b
			},
		},
		{
			Module:   "math",
			Function: "multiply",
			Handler: func(a, b uint32) uint32 {
				return a * b
			},
		},

		// === Byte-Based Host Functions (flexible, like plugins) ===
		{
			Module:   "native_host",
			Function: "uppercase",
			Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
				return 0, bytes.ToUpper(input)
			}),
		},
		{
			Module:   "native_host",
			Function: "reverse",
			Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
				runes := []rune(string(input))
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return 0, []byte(string(runes))
			}),
		},
		{
			Module:   "native_host",
			Function: "echo",
			Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
				message := fmt.Sprintf("[NATIVE_HOST_ECHO] %s", string(input))
				return 0, []byte(message)
			}),
		},
		{
			Module:   "native_host",
			Function: "timestamp",
			Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
				timestamp := time.Now().Format("2006-01-02 15:04:05.000")
				return 0, []byte(timestamp)
			}),
		},
	}))
	defer manager.Close()

	fmt.Print("=== WASM Plugin Communication Demo ===\n\n")

	// ===== Part 1: Native Host Functions (Primitive Types) =====
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Part 1: Native Host Functions (Primitive Types)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n1. Testing mathtest plugin (calls math.add & math.multiply):")
	returnValue, result := Must2(manager.Call("mathtest", "calculate", []byte{}))
	fmt.Printf("   Return: %d, Output: '%s'\n", returnValue, string(result))
	fmt.Println("   ℹ️  This plugin uses direct WASM imports to call math.add and math.multiply")
	fmt.Println("   ℹ️  Zero-copy performance for primitive type operations")

	// ===== Part 2: Native Host Functions (Byte-Based) =====
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Part 2: Native Host Functions (Byte-Based)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n2. Testing native_host.uppercase:")
	returnValue, result = Must2(manager.Call("native_host", "uppercase", []byte("hello world")))
	fmt.Printf("   Input: 'hello world'\n")
	fmt.Printf("   Return: %d, Output: '%s'\n", returnValue, string(result))

	fmt.Println("\n3. Testing native_host.reverse:")
	returnValue, result = Must2(manager.Call("native_host", "reverse", []byte("WASM Plugin")))
	fmt.Printf("   Input: 'WASM Plugin'\n")
	fmt.Printf("   Return: %d, Output: '%s'\n", returnValue, string(result))

	fmt.Println("\n4. Testing native_host.echo:")
	returnValue, result = Must2(manager.Call("native_host", "echo", []byte("Testing native host")))
	fmt.Printf("   Return: %d, Output: '%s'\n", returnValue, string(result))

	fmt.Println("\n5. Testing native_host.timestamp:")
	returnValue, result = Must2(manager.Call("native_host", "timestamp", []byte{}))
	fmt.Printf("   Return: %d, Timestamp: '%s'\n", returnValue, string(result))

	// ===== Part 3: Plugin-based Host Functions (for comparison) =====
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Part 3: Plugin-Based Host Functions (WASM)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n6. Testing host.print (WASM plugin):")
	returnValue, result = Must2(manager.Call("host", "print", []byte("Hello from main!")))
	fmt.Printf("   Return: %d, Output: '%s'\n", returnValue, string(result))

	fmt.Println("\n7. Testing host.get_timestamp (WASM plugin):")
	returnValue, result = Must2(manager.Call("host", "get_timestamp", []byte{}))
	fmt.Printf("   Return: %d, Timestamp: '%s'\n", returnValue, string(result))

	// ===== Part 4: Plugin-to-Plugin Communication =====
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Part 4: Plugin-to-Plugin Communication")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n8. Testing logger plugin (logger → host.print):")
	returnValue, result = Must2(manager.Call("logger", "log", []byte("Test message")))
	fmt.Printf("   Return: %d, Output: '%s'\n", returnValue, string(result))

	fmt.Println("\n9. Testing full chain (greet → logger → host.print):")
	returnValue, result = Must2(manager.Call("greet", "greet", []byte("World")))
	fmt.Printf("   Return: %d, Output: '%s'\n", returnValue, string(result))

	fmt.Println("\n10. Multiple greetings (showing randomness):")
	for i := 0; i < 3; i++ {
		_, result = Must2(manager.Call("greet", "greet", []byte(fmt.Sprintf("User%d", i+1))))
		fmt.Printf("    Greeting %d: %s\n", i+1, string(result))
	}

	// ===== Summary =====
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Summary: Host Function Approaches")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("\n✅ Primitive Host Functions (math.add, math.multiply)")
	fmt.Println("   • Zero-copy, direct function calls")
	fmt.Println("   • Best for: Math, simple logic, performance-critical code")
	fmt.Println("   • Called from plugins via direct WASM imports")

	fmt.Println("\n✅ Byte-Based Native Host Functions (native_host.*)")
	fmt.Println("   • Work with []byte input/output like plugins")
	fmt.Println("   • Best for: String processing, complex data, Go library integration")
	fmt.Println("   • Written in native Go, no WASM compilation needed")

	fmt.Println("\n✅ Plugin-Based Host Functions (host.wasm)")
	fmt.Println("   • Implemented as WASM plugins")
	fmt.Println("   • Best for: When host functions need the same isolation as plugins")
	fmt.Println("   • Can be hot-reloaded like any plugin")

	fmt.Println("\n=== Demo Complete ===")
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

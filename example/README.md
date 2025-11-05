# WASM Plugin System - Example

This directory contains example plugins demonstrating the WASM plugin system with **plugin-to-plugin communication**.

## Features Demonstrated

- ✅ **Native Host Functions** (primitive types & byte-based)
- ✅ Host → Plugin calls
- ✅ Plugin → Plugin calls (using `pdk.Call()`)
- ✅ Plugin → Native Host Function calls
- ✅ Direct WASM imports for performance-critical functions
- ✅ Multi-language plugins (Go via TinyGo, Zig)
- ✅ Call depth limiting and safety
- ✅ Context switching for nested calls

## Plugin Architecture

```
Host (Go)
  ↓
greet.wasm (Go)
  ├→ random.wasm (Zig) - Direct WASM import for RNG
  └→ logger.wasm (Go) - Plugin call via pdk.Call()
      └→ host.wasm (Go) - Plugin call to "host" services
```

## Prerequisites

- Go 1.19+
- TinyGo (for building Go plugins)
- Zig (for building Zig plugins)

## Quick Start

### 1. Build all plugins:

```bash
cd example

# Build Go plugins with TinyGo
GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o host.wasm ./plugins/host/main.go
GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o logger.wasm ./plugins/logger/main.go
GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o greet.wasm ./plugins/greet/main.go
GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o mathtest.wasm ./plugins/mathtest/main.go

# Build Zig plugin
zig build-exe ./plugins/random/main.zig -target wasm32-freestanding -fno-entry -rdynamic -O ReleaseFast -femit-bin=random.wasm

# Move WASM files to app directory
mv *.wasm cmd/app/
```

### 2. Run the demo:

```bash
cd cmd/app
go run main.go
```

### 3. Expected Output:

```
=== WASM Plugin Communication Demo ===

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Part 1: Native Host Functions (Primitive Types)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Testing math.add (native host function):
   ℹ️  Primitive functions like math.add are designed to be called from plugins
   ℹ️  They work with simple types (uint32, float32, etc.) for zero-copy performance

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Part 2: Native Host Functions (Byte-Based)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

2. Testing native_host.uppercase:
   Input: 'hello world'
   Return: 0, Output: 'HELLO WORLD'

3. Testing native_host.reverse:
   Input: 'WASM Plugin'
   Return: 0, Output: 'nigulP MSAW'

4. Testing native_host.echo:
   Return: 0, Output: '[NATIVE_HOST_ECHO] Testing native host'

5. Testing native_host.timestamp:
   Return: 0, Timestamp: '2025-11-05 14:30:45.123'

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Part 3: Plugin-Based Host Functions (WASM)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

6. Testing host.print (WASM plugin):
   Return: 0, Output: 'printed'

7. Testing host.get_timestamp (WASM plugin):
   Return: 0, Timestamp: '2025-11-05 14:30:45'

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Part 4: Plugin-to-Plugin Communication
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

8. Testing logger plugin (logger → host.print):
   Return: 0, Output: '[LOG] Test message'

9. Testing full chain (greet → logger → host.print):
   Return: 0, Output: 'Welcome, World! | Logger says: [LOG] Generated greeting for World'

10. Multiple greetings (showing randomness):
    Greeting 1: Salutations, User1! | Logger says: [LOG] Generated greeting for User1
    Greeting 2: Hi, User2! | Logger says: [LOG] Generated greeting for User2
    Greeting 3: Hey there, User3! | Logger says: [LOG] Generated greeting for User3

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Summary: Host Function Approaches
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Primitive Host Functions (math.add, math.multiply)
   • Zero-copy, direct function calls
   • Best for: Math, simple logic, performance-critical code
   • Called from plugins via direct WASM imports

✅ Byte-Based Native Host Functions (native_host.*)
   • Work with []byte input/output like plugins
   • Best for: String processing, complex data, Go library integration
   • Written in native Go, no WASM compilation needed

✅ Plugin-Based Host Functions (host.wasm)
   • Implemented as WASM plugins
   • Best for: When host functions need the same isolation as plugins
   • Can be hot-reloaded like any plugin

=== Demo Complete ===
```

## Host Functions vs Plugins

This example demonstrates **three different approaches** for exposing functionality to plugins:

### 1. Native Host Functions (Primitive Types) - `math.*`
**Implementation:** Native Go code with simple types
```go
sdk.HostFunction{
    Module: "math",
    Function: "add",
    Handler: func(a, b uint32) uint32 { return a + b },
}
```
**Use case:** Math operations, boolean logic, performance-critical simple operations  
**Advantages:** Zero-copy, fastest possible, no WASM compilation needed  
**Limitations:** Only primitive types (i32, i64, f32, f64)

### 2. Native Host Functions (Byte-Based) - `native_host.*`
**Implementation:** Native Go code with []byte interface
```go
sdk.HostFunction{
    Module: "native_host",
    Function: "uppercase",
    Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
        return 0, bytes.ToUpper(input)
    }),
}
```
**Use case:** String processing, JSON handling, database queries, API calls  
**Advantages:** Full Go ecosystem access, easy to write, no WASM compilation  
**Limitations:** Memory copies for input/output (still very fast)

### 3. Plugin-Based Host Functions - `host.wasm`
**Implementation:** WASM plugin (compiled from Go/Zig/Rust)
```go
//export print
func Print() uint32 {
    input := pdk.Input()
    fmt.Println("[HOST]", string(input))
    pdk.Output([]byte("printed"))
    return 0
}
```
**Use case:** When you need the same isolation/sandboxing for host functions  
**Advantages:** Hot-reloadable, sandboxed, same interface as other plugins  
**Limitations:** Requires WASM compilation, slightly slower

## Plugin Descriptions

### `host.wasm` (Plugin-Based Host Functions)
Provides "host services" as a WASM plugin (for demonstration):
- `print(message)` - Print to console (simulated)
- `get_timestamp()` - Get current timestamp

**Note:** This demonstrates implementing host functions as plugins. In production, you would typically use native Go host functions (primitive or byte-based) for better performance.

### `random.wasm` (Zig)
Provides random number generation:
- `next()` - Returns a random float32

**Called by:** `greet.wasm` using direct WASM import (`//go:wasmimport`)

### `logger.wasm` (Go)
Logging service:
- `log(message)` - Formats and logs messages
- Calls `host.print()` using `pdk.Call()`

**Called by:** `greet.wasm` using `pdk.Call()`

### `greet.wasm` (Go)
Main greeting plugin demonstrating multiple communication patterns:
- Uses **direct WASM import** to call `random.next()` for performance
- Uses **pdk.Call()** to call `logger.log()` for flexibility
- Demonstrates the full plugin communication chain

### `mathtest.wasm` (Go) 🆕
Demonstrates calling native primitive host functions:
- Uses **direct WASM import** to call `math.add()` and `math.multiply()`
- Also calls byte-based native host functions via `pdk.Call()`
- Shows how plugins can mix both primitive and byte-based host function calls

## Communication Patterns

### 1. Native Host Functions (Primitive Types) 🚀 NEW!
For zero-copy, high-performance operations with simple types:

```go
// Register in host application
sdk.HostFunction{
    Module:   "math",
    Function: "add",
    Handler: func(a, b uint32) uint32 {
        return a + b
    },
}

// Call from plugin via direct WASM import
//go:wasmimport math add
func add(a, b uint32) uint32

result := add(5, 3) // Returns 8
```

**Pros:** Zero-copy, minimal overhead, direct function call, native Go code  
**Cons:** Only supports primitive WASM types (i32, i64, f32, f64)  
**Best for:** Math operations, simple logic, performance-critical paths

### 2. Native Host Functions (Byte-Based) 🚀 NEW!
For flexible data exchange with native Go code:

```go
// Register in host application
sdk.HostFunction{
    Module:   "native_host",
    Function: "uppercase",
    Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
        return 0, bytes.ToUpper(input)
    }),
}

// Call from plugin using pdk.Call()
import "github.com/justgook/wpm/pdk"

_, output, err := pdk.Call("native_host", "uppercase", []byte("hello"))
// output: "HELLO"
```

**Pros:** Works with []byte like plugins, native Go code (no WASM compilation), easy library integration, host can monitor/control  
**Cons:** Slightly higher overhead than primitive types due to memory copies  
**Best for:** String processing, complex data structures, integrating Go libraries

### 3. Direct WASM Import (Fast Path)
For simple, frequently-called functions with primitive types:

```go
//go:wasmimport random next
func nextRand() float32

// Call it directly
value := nextRand()
```

**Pros:** Minimal overhead, direct function call  
**Cons:** Only supports primitive WASM types, host can't intercept

### 4. PDK Call (Flexible Path)
For complex data exchange and composability:

```go
import "github.com/justgook/wpm/pdk"

// Call another plugin
_, output, err := pdk.Call("logger", "log", []byte("message"))

// Call host function (works with both native and WASM-based host functions)
_, output, err := pdk.Call("native_host", "uppercase", []byte("hello"))
```

**Pros:** Works with any data ([]byte), host can monitor/control, flexible, works with both native and WASM host functions  
**Cons:** Slightly higher overhead due to manager mediation

## Key Configuration

```go
manager := sdk.New(ctx, sdk.Config{
    EnableWASI:   true,        // Enable WASI support
    MaxCallDepth: 10,          // Prevent infinite recursion
}, modules, []sdk.HostFunction{
    // Primitive type host functions
    {
        Module:   "math",
        Function: "add",
        Handler: func(a, b uint32) uint32 { return a + b },
    },
    // Byte-based host functions
    {
        Module:   "native_host",
        Function: "uppercase",
        Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
            return 0, bytes.ToUpper(input)
        }),
    },
})
```

## Calling Native Host Functions from Plugins

### Calling Primitive Host Functions

Use direct WASM imports for zero-copy performance:

```go
package main

import "github.com/justgook/wpm/pdk"

// Import primitive host functions
//go:wasmimport math add
func mathAdd(a, b uint32) uint32

//go:wasmimport math multiply
func mathMultiply(a, b uint32) uint32

//export calculate
func Calculate() uint32 {
    sum := mathAdd(5, 3)        // Direct call, zero-copy
    product := mathMultiply(4, 7) // Direct call, zero-copy
    
    output := fmt.Sprintf("Sum: %d, Product: %d", sum, product)
    pdk.Output([]byte(output))
    return 0
}
```

### Calling Byte-Based Host Functions

Use `pdk.Call()` for flexible data exchange:

```go
package main

import "github.com/justgook/wpm/pdk"

//export process
func Process() uint32 {
    input := pdk.Input()
    
    // Call native host function
    _, upperResult, err := pdk.Call("native_host", "uppercase", input)
    if err != nil {
        return 1
    }
    
    // Call another native host function
    _, reversed, _ := pdk.Call("native_host", "reverse", upperResult)
    
    pdk.Output(reversed)
    return 0
}
```

## API Reference

### PDK (Plugin Development Kit)

```go
// Core functions (existing)
pdk.Input() []byte          // Get input from caller
pdk.Output(data []byte)     // Set output for caller
pdk.Alloc(size uint64) uint32
pdk.Free(ptr uint32)

// New: Plugin-to-plugin communication
pdk.Call(module, function string, input []byte) (int32, []byte, error)
pdk.CallPlugin(plugin, function string, input []byte) (int32, []byte, error)
pdk.CallHost(function string, input []byte) (int32, []byte, error)
```

### SDK (Host Application)

```go
manager.Call(moduleName, functionName string, input []byte) (int32, []byte, error)
```

## Architecture Notes

1. **Call Stack Management:** The manager maintains a call stack to prevent infinite recursion and track context
2. **Context Switching:** Input/output state is preserved when switching between plugins
3. **Safety:** Call depth is limited (default: 10 levels)
4. **Performance:** Direct WASM imports bypass the manager for performance-critical paths

## Building for Production

### Choosing the Right Approach

| Feature | Primitive Host Functions | Byte-Based Host Functions | Plugin-Based |
|---------|-------------------------|---------------------------|--------------|
| **Performance** | ⚡⚡⚡ Fastest | ⚡⚡ Fast | ⚡ Good |
| **Ease of Development** | ✅ Very Easy | ✅✅ Easiest | ⚠️ Requires compilation |
| **Data Flexibility** | ⚠️ Primitives only | ✅✅ Any []byte data | ✅✅ Any []byte data |
| **Go Library Access** | ✅✅ Full access | ✅✅ Full access | ⚠️ Limited (TinyGo) |
| **Hot Reload** | ❌ Requires restart | ❌ Requires restart | ✅ Can reload |
| **Isolation/Sandbox** | ❌ Runs in host | ❌ Runs in host | ✅ WASM sandbox |
| **Best For** | Math, flags, counters | Strings, JSON, DB, APIs | User plugins, untrusted code |

### Recommendations

1. **Use Primitive Host Functions for:**
   - Mathematical operations (`add`, `multiply`, `sin`, `cos`)
   - Boolean logic and flags
   - Performance-critical counters and metrics
   - Simple state queries

2. **Use Byte-Based Native Host Functions for:**
   - String processing (`uppercase`, `lowercase`, `regex`)
   - JSON encoding/decoding
   - Database queries
   - HTTP API calls
   - File system operations
   - Most business logic

3. **Use Plugin-Based Host Functions for:**
   - When you need hot-reloadability for host services
   - Untrusted code that needs sandboxing
   - Multi-language implementations
   - When you want the same interface for all plugins

### Additional Production Considerations

1. **Error Handling:** Implement proper error propagation and logging
2. **Security:** Add permission systems to control which plugins can call which functions
3. **Monitoring:** Add telemetry to track plugin call chains and performance
4. **Rate Limiting:** Protect expensive host functions with rate limits
5. **Caching:** Cache results of expensive host function calls when appropriate

## Troubleshooting

**Plugin not found errors:**
- Ensure all .wasm files are in the `cmd/app/` directory
- Check that module names match in both the SDK initialization and plugin calls

**Call depth exceeded:**
- Increase `MaxCallDepth` in config
- Check for circular dependencies (A→B→A)

**Build errors:**
- Verify TinyGo and Zig are installed and in PATH
- Use the exact build commands shown above

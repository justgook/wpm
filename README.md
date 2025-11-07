# 🧩 Plugin ABI Specification (v1.0 Draft)

This document defines the **host ↔ plugin binary interface** for lightweight WebAssembly plugins used in the game engine.  
The goal is to keep the ABI minimal, stable, and composable — focused on predictable byte-level data exchange and memory reuse.

---

## Overview

Each plugin runs in a WebAssembly environment instantiated by the host application.  
The host provides a small set of imported functions to allow the plugin to:

- Allocate memory in the shared linear memory space.
    
- Access input data provided by the host.
    
- Declare an output region the host can read after execution.
    

A plugin may export any number of callable functions (e.g., event handlers, systems, etc.).  
Each call represents a discrete **input → process → output** step.

---

## 🧭 ABI Version

|Field|Value|
|---|---|
|**ABI Version**|`1.0`|
|**Target**|WebAssembly 1.0 + host runtime (e.g. wazero)|
|**Memory model**|Shared linear memory (single instance)|

---

## 🧠 Core Host Imports

The following functions **must be provided by the host** and are available to every plugin.

|Function|Signature|Description|
|---|---|---|
|`alloc(size: u64) → u32`|Allocates `size` bytes in host-managed memory and returns the offset (pointer).||
|`free(ptr: u32)`|Frees memory previously allocated with `alloc`.||
|`input_ptr() → u32`|Returns the offset of the current input buffer.||
|`input_len() → u32`|Returns the length (in bytes) of the current input buffer.||
|`set_output(ptr: u32, len: u32)`|Informs the host where the plugin’s output data will be stored. The host reads this region after the call returns.||

---

## 📦 Plugin Responsibilities

- **Export functions**  
    The plugin can export any number of callable functions.  
    Function signatures are up to the developer; typically, no arguments are required since input/output are passed via memory.
    
- **Input access**  
    The plugin retrieves input data using `input_ptr()` / `input_len()` and interprets it according to its own protocol.
    
- **Memory usage**  
    The plugin uses `alloc()` / `free()` to manage temporary or persistent state buffers.  
    It may keep allocated regions alive across calls for caching or incremental updates.
    
- **Output registration**  
    The plugin calls `set_output(ptr, len)` once to register its output buffer.  
    After that, it can reuse the same buffer and simply overwrite its contents between calls.
    
- **Return values**  
    The integer return value of a plugin function is developer-defined.  
    Conventionally:
    
    - `0` = success (host interprets output normally)
        
    - Non-zero = error (host may read output as error message)  
        But this is **only a suggestion**, not enforced by the ABI.
        

---

## 🔄 Call Sequence

1. **Host prepares input** in memory accessible to `input_ptr()` / `input_len()`.
    
2. **Host calls** one exported plugin function.
    
3. **Plugin execution:**
    
    - Reads input buffer.
        
    - Optionally allocates or reuses working/state memory.
        
    - Writes results into its output buffer.
        
    - Optionally calls `set_output(ptr, len)` if the buffer location changes.
        
4. **Plugin returns.**
    
5. **Host checks return value** and reads from the region specified by the last `set_output` call.
    

---

## 🧩 Design Principles

- **Simplicity > abstraction** – all data exchange is via raw bytes.
    
- **Stable memory contract** – avoid re-registering outputs unnecessarily.
    
- **No required runtime** – plugins can be called directly without event loops or schedulers.
    
- **Developer-defined semantics** – ABI defines _where_ bytes live, not _what_ they mean.
    

---

## 🚧 Future Extensions (Proposed for v2)

These extensions are **not** part of the v1 ABI but are noted for forward compatibility.

### 1. `alloc_shared(tag: u32, size: u64) → u32`

Allocate or retrieve a persistent, named shared memory region managed by the host.  
Intended for cases where multiple plugins share common data or cached render pipelines.

### 2. Enhanced Output Registration

Future versions may introduce multiple output “slots”:

`set_output(id: u32, ptr: u32, len: u32)`

Allowing a single plugin to register several logical outputs (e.g., `state`, `commands`, `diagnostics`).

### 3. Optional Debug/Error Channels

A dedicated error or log buffer could be standardized for better tooling support.  
Current convention (`return != 0`) remains valid.

---

## 🧱 Example Memory Lifecycle (Typical)

1. **Initialization**
    
    - Plugin allocates state buffers using `alloc(size)`.
        
    - Registers output with `set_output(ptr, len)`.
        
2. **Per-Frame Update**
    
    - Host updates input region.
        
    - Calls `update()` exported from plugin.
        
    - Plugin updates its internal state and writes new draw commands into output buffer.
        
3. **Host Consumption**
    
    - Reads output buffer.
        
    - If return != 0, interprets it as an error.
        
4. **Shutdown**
    
    - Host may call plugin cleanup or free buffers manually via `free(ptr)` if needed.
        

---

## 🔒 Compatibility Notes

- Host and plugin must agree on **endianness** (WASM standard = little-endian).
    
- All offsets and lengths are expressed in **bytes** relative to the shared linear memory.
    
- The ABI is designed to remain compatible with pure WASM runtimes like **wazero**.

---

## 💻 Language Support

This ABI is language-agnostic and can be implemented in any language that compiles to WebAssembly.

### Official PDK Implementations

#### Go PDK (`pdk/pdk.go`)

High-level Go library for building plugins with TinyGo:

```go
import "github.com/justgook/wpm/pdk"

//export greet
func Greet() uint32 {
    input := pdk.Input()
    name := string(input)
    greeting := "Hello, " + name + "!"
    pdk.Output([]byte(greeting))
    return 0
}
```

**Features:**
- Automatic memory management
- Idiomatic Go interfaces
- Plugin-to-plugin calls via `pdk.Call()`
- String/slice convenience wrappers

**Build:** Requires [TinyGo](https://tinygo.org/)

#### C PDK (`pdk/pdk.h`)

Single-header library for building plugins in C:

```c
#include "pdk.h"

__attribute__((export_name("greet")))
uint32_t greet(void) {
    uint32_t input_len;
    const uint8_t *input = pdk_input(&input_len);
    
    // Process input...
    
    pdk_output(output, output_len);
    return 0;
}
```

**Features:**
- Header-only (no separate compilation)
- Zero libc dependencies
- Custom `pdk_strlen()` and `pdk_memcpy()`
- Both string and explicit-length API variants
- Plugin-to-plugin calls via `pdk_call()` / `pdk_call_str()`

**Build:** Requires [WASI SDK](https://github.com/WebAssembly/wasi-sdk) or clang with WebAssembly support

### Official SDK Implementations

#### Go SDK (`sdk/sdk.go`)

Server-side SDK for loading and managing WASM plugins in Go applications:

```go
import "github.com/justgook/wpm/sdk"

manager, _ := sdk.New(ctx, sdk.Config{
    EnableWASI: true,
}, []sdk.Module{
    {Name: "greet", WasmData: wasmBytes},
}, []sdk.HostFunction{
    {
        Module: "host",
        Function: "print",
        Handler: sdk.ByteHandler(func(input []byte) (int32, []byte) {
            fmt.Println(string(input))
            return 0, []byte{}
        }),
    },
})

returnCode, output, _ := manager.Call("greet", "greet", []byte("World"))
```

**Features:**
- Uses wazero runtime
- Full WASI support
- Plugin-to-plugin calls
- Host function registration
- Production-ready

**Build:** Requires Go 1.21+

#### JavaScript SDK (`sdk-js/wasm-plugin-sdk.js`)

Browser SDK for loading and running WASM plugins in web applications:

```javascript
const manager = await PluginManager.create({
  modules: [
    { name: 'greet', url: '/plugins/greet.wasm' }
  ],
  hostFunctions: [
    {
      module: 'host',
      function: 'print',
      handler: (input) => {
        console.log(new TextDecoder().decode(input));
        return { returnCode: 0, output: new Uint8Array() };
      }
    }
  ]
});

const result = await manager.call('greet', 'greet', 'World');
```

**Features:**
- Zero dependencies
- Single file (~450 lines)
- Works with same WASM files as Go SDK
- Full plugin-to-plugin call support
- Native browser WebAssembly API

**Build:** No build required, works in all modern browsers

### SDK Comparison

| Feature | Go SDK | JavaScript SDK |
|---------|--------|----------------|
| **Runtime** | wazero | WebAssembly API |
| **Environment** | Server/CLI | Browser |
| **Performance** | ⚡⚡⚡ Very Fast | ⚡⚡ Fast |
| **WASI Support** | ✅ Full | ⚠️ Limited |
| **Plugin Compatibility** | ✅ All | ✅ All |
| **File Size** | Binary | ~450 lines JS |

### Plugin Language Comparison

| Feature | Go (TinyGo) | C |
|---------|-------------|---|
| **Memory Safety** | ✅ Automatic GC | ⚠️ Manual management |
| **String Handling** | ✅ Native strings | ⚠️ Manual byte arrays |
| **Binary Size** | ~100-500KB | ~1-10KB |
| **Performance** | ⚡⚡ Fast | ⚡⚡⚡ Fastest |
| **Dev Experience** | ✅✅ Excellent | ⚠️ Manual |
| **libc Required** | ❌ No | ❌ No (custom utils) |
| **Build Complexity** | Low (TinyGo) | Low (WASI SDK) |

### Examples

See `example/plugins/` for working examples in multiple languages:
- `logger/` - Go implementation
- `logger_c/` - C implementation (functionally identical)
- `greet/` - Go with plugin-to-plugin calls
- `random/` - Zig implementation

All plugins share the same ABI and can call each other regardless of implementation language.
    

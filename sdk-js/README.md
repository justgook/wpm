# WASM Plugin SDK for JavaScript (Browser)

JavaScript/Browser SDK for loading and managing WebAssembly plugins. Compatible with plugins built using the Go PDK or C PDK.

## Features

- 🌐 **Browser-first**: Works in any modern browser
- 📦 **Single file**: No build step required, just include the script
- 🔌 **Plugin compatible**: Works with WASM plugins built with Go PDK or C PDK
- 🔄 **Plugin-to-plugin calls**: Plugins can call each other
- 🎯 **Host functions**: Register native JavaScript functions callable from WASM
- ⚡ **Zero dependencies**: Pure JavaScript, no external libraries
- 🌍 **WASI support**: Built-in WASI polyfill for TinyGo/WASI plugins

## Quick Start

### 1. Include the SDK

```html
<script src="wasm-plugin-sdk.js"></script>
```

### 2. Create a Plugin Manager

```javascript
const manager = await PluginManager.create({
  modules: [
    { name: 'logger', url: '/plugins/logger.wasm' },
    { name: 'greet', url: '/plugins/greet.wasm' }
  ],
  hostFunctions: [
    {
      module: 'host',
      function: 'print',
      handler: (input) => {
        const text = new TextDecoder().decode(input);
        console.log(text);
        return { returnCode: 0, output: new Uint8Array() };
      }
    }
  ]
});
```

### 3. Call Plugin Functions

```javascript
// Call with string input
const result = await manager.call('greet', 'greet', 'World');
console.log(new TextDecoder().decode(result.output));

// Call with byte input
const input = new TextEncoder().encode('Hello');
const result2 = await manager.call('logger', 'log', input);
```

## API Reference

### PluginManager.create(options)

Creates a new plugin manager instance.

**Options:**
- `modules`: Array of modules to load (loaded in order)
  - `name`: Module name (string)
  - `url`: URL to WASM file (optional)
  - `data`: ArrayBuffer with WASM data (optional)
  - **Important**: Modules are loaded sequentially in the order specified. If a module imports functions from another module (via `//go:wasmimport` or similar), the imported module must be listed first.
- `hostFunctions`: Array of host functions
  - `module`: Module name (string)
  - `function`: Function name (string)
  - `handler`: Function handler `(input: Uint8Array) => { returnCode: number, output: Uint8Array }`
- `config`: Configuration options
  - `envModuleName`: Name of env module (default: 'env')
  - `maxCallDepth`: Max plugin-to-plugin call depth (default: 10)

**Example:**

```javascript
const manager = await PluginManager.create({
  modules: [
    // Load dependencies first
    { name: 'random', url: '/plugins/random.wasm' },
    { name: 'logger', url: '/plugins/logger.wasm' },
    // Then modules that depend on them
    { name: 'greet', url: '/plugins/greet.wasm' } // imports from 'random'
  ],
  hostFunctions: [
    {
      module: 'utils',
      function: 'log',
      handler: (input) => {
        console.log(new TextDecoder().decode(input));
        return { returnCode: 0, output: new Uint8Array() };
      }
    }
  ],
  config: {
    maxCallDepth: 20
  }
});
```

### manager.call(moduleName, functionName, input)

Calls a function in a loaded module.

**Parameters:**
- `moduleName`: Name of the module (string)
- `functionName`: Name of the exported function (string)
- `input`: Input data (string or Uint8Array)

**Returns:** `Promise<{ returnCode: number, output: Uint8Array }>`

**Example:**

```javascript
// String input
const result1 = await manager.call('greet', 'greet', 'Alice');

// Byte input
const input = new TextEncoder().encode('Bob');
const result2 = await manager.call('greet', 'greet', input);

// Access return code and output
console.log('Return code:', result1.returnCode);
console.log('Output:', new TextDecoder().decode(result1.output));
```

### manager.loadWasmModule(module)

Dynamically load a new WASM module after manager creation.

**Parameters:**
- `module`: Object with `name` and either `url` or `data`

**Example:**

```javascript
// Load from URL
await manager.loadWasmModule({
  name: 'newPlugin',
  url: '/plugins/newPlugin.wasm'
});

// Load from ArrayBuffer (e.g., file upload)
const fileBuffer = await file.arrayBuffer();
await manager.loadWasmModule({
  name: 'uploadedPlugin',
  data: fileBuffer
});
```

### manager.close()

Cleanup and release resources.

**Example:**

```javascript
manager.close();
```

## Host Functions

Host functions are native JavaScript functions that can be called from WASM plugins.

### Byte Handler (Recommended)

Works with byte arrays, similar to plugin functions:

```javascript
{
  module: 'utils',
  function: 'uppercase',
  handler: (input) => {
    const text = new TextDecoder().decode(input);
    const upper = text.toUpperCase();
    return {
      returnCode: 0,
      output: new TextEncoder().encode(upper)
    };
  }
}
```

From a plugin (Go):
```go
_, output, _ := pdk.Call("utils", "uppercase", []byte("hello"))
// output = "HELLO"
```

From a plugin (C):
```c
pdk_call_result_t result = pdk_call_str("utils", "uppercase", 
                                         (uint8_t*)"hello", 5);
// result.output = "HELLO"
```

### Primitive Handler

For simple numeric operations (zero-copy):

```javascript
{
  module: 'math',
  function: 'add',
  isByteHandler: false,
  handler: (a, b) => a + b
}
```

From a plugin (Go):
```go
//go:wasmimport math add
func add(a, b uint32) uint32
```

## Complete Example

### HTML Page

```html
<!DOCTYPE html>
<html>
<head>
  <title>WASM Plugin Demo</title>
  <script src="wasm-plugin-sdk.js"></script>
</head>
<body>
  <h1>WASM Plugin Demo</h1>
  <button onclick="runDemo()">Run Demo</button>
  <pre id="output"></pre>

  <script>
    async function runDemo() {
      const output = document.getElementById('output');
      output.textContent = 'Loading plugins...\n';

      // Create plugin manager
      const manager = await PluginManager.create({
        modules: [
          { name: 'greet', url: '/plugins/greet.wasm' },
          { name: 'logger', url: '/plugins/logger.wasm' }
        ],
        hostFunctions: [
          {
            module: 'host',
            function: 'print',
            handler: (input) => {
              const text = new TextDecoder().decode(input);
              output.textContent += `[HOST] ${text}\n`;
              return { returnCode: 0, output: new Uint8Array() };
            }
          },
          {
            module: 'utils',
            function: 'timestamp',
            handler: (input) => {
              const ts = new Date().toISOString();
              return { 
                returnCode: 0, 
                output: new TextEncoder().encode(ts) 
              };
            }
          }
        ]
      });

      output.textContent += 'Plugins loaded!\n\n';

      // Call greet plugin
      const result = await manager.call('greet', 'greet', 'Browser');
      const greeting = new TextDecoder().decode(result.output);
      output.textContent += `Greeting: ${greeting}\n`;

      // Get timestamp from host function
      const tsResult = await manager.call('utils', 'timestamp', '');
      const timestamp = new TextDecoder().decode(tsResult.output);
      output.textContent += `Timestamp: ${timestamp}\n`;

      manager.close();
    }
  </script>
</body>
</html>
```

### Running the Example

1. Make sure your WASM plugins are built:
```bash
cd example
./build.sh
```

2. Start a local web server from the project root:
```bash
python3 -m http.server 8000
```

3. Open your browser to:
```
http://localhost:8000/sdk-js/example.html
```

4. Click "Load Default Plugins" and "Run All Tests"

## Building Compatible Plugins

### Go Plugin Example

Using the Go PDK (`pdk/pdk.go`):

```go
package main

import "github.com/justgook/wpm/pdk"

//export greet
func Greet() uint32 {
    input := pdk.Input()
    name := string(input)
    greeting := "Hello, " + name + "!"
    pdk.Output([]byte(greeting))
    return 0
}

func main() {}
```

Build with TinyGo:
```bash
tinygo build -o greet.wasm -target=wasi -no-debug main.go
```

### C Plugin Example

Using the C PDK (`pdk/pdk.h`):

```c
#include "pdk.h"

__attribute__((export_name("greet")))
uint32_t greet(void) {
    uint32_t input_len;
    const uint8_t *input = pdk_input(&input_len);
    
    // Process input...
    const char *output = "Hello from C!";
    
    pdk_output((uint8_t*)output, 14);
    return 0;
}
```

Build with clang:
```bash
clang --target=wasm32-wasi -nostdlib -O3 \
      -Wl,--no-entry -Wl,--export=greet \
      -o greet.wasm main.c
```

## Architecture

The JavaScript SDK mirrors the Go SDK's architecture:

```
┌─────────────────────────────────────────┐
│         Browser Application             │
│  (HTML + JavaScript using SDK)          │
└───────────────┬─────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────┐
│       JavaScript SDK (PluginManager)    │
│  - Module loading                       │
│  - Memory management (alloc/free)       │
│  - Host function registry               │
│  - Plugin-to-plugin calls               │
└───┬─────────────────────────────────┬───┘
    │                                 │
    ▼                                 ▼
┌──────────────────┐      ┌──────────────────┐
│  WASM Plugins    │      │  Host Functions  │
│  (Go, C, Zig)    │◄────►│  (JavaScript)    │
└──────────────────┘      └──────────────────┘
```

### Communication Flow

1. **Host → Plugin**: `manager.call()` writes input to WASM memory, calls exported function
2. **Plugin → Plugin**: Plugin uses `pdk.Call()` → env.plugin_call → manager routes to target
3. **Plugin → Host**: Plugin uses `pdk.Call()` → manager finds host function → executes JS
4. **Output**: Plugin calls `pdk.Output()` → env.set_output → manager reads from memory

## Compatibility

### WASM Plugin Compatibility

Works with any WASM plugin built using:
- ✅ Go PDK (`pdk/pdk.go`) with TinyGo
- ✅ C PDK (`pdk/pdk.h`) with WASI SDK or clang
- ✅ Any language that exports functions and uses the standard ABI

The same WASM files work in both:
- Go SDK (server-side with wazero)
- JavaScript SDK (browser with WebAssembly API)

### WASI Support

The SDK includes a built-in WASI polyfill for plugins compiled with WASI support (TinyGo uses WASI by default). The polyfill provides:

- ✅ Basic WASI functions (fd_write, fd_read, etc.)
- ✅ Random number generation (using browser's crypto API)
- ✅ Clock/time functions
- ✅ Environment and arguments (empty by default)
- ⚠️  File system operations return errors (browser security)
- ⚠️  Network operations not supported

This means TinyGo plugins work out of the box without any modifications!

### Browser Support

Requires WebAssembly support:
- ✅ Chrome 57+ (March 2017)
- ✅ Firefox 52+ (March 2017)
- ✅ Safari 11+ (September 2017)
- ✅ Edge 16+ (October 2017)

All modern browsers are supported!

## Differences from Go SDK

While the JavaScript SDK mirrors the Go SDK's API, there are some differences:

| Feature | Go SDK | JavaScript SDK |
|---------|--------|----------------|
| **Runtime** | wazero | WebAssembly API |
| **Environment** | Server/CLI | Browser |
| **Module Loading** | File system | fetch() / File API |
| **String Encoding** | Native | TextEncoder/TextDecoder |
| **Memory Model** | Direct access | Uint8Array views |
| **WASI Support** | Full | Browser limitations |
| **Performance** | Very fast | Fast (browser dependent) |

## Troubleshooting

### Plugins fail to load

Make sure you're serving from a web server (not `file://`):
```bash
python3 -m http.server 8000
```

### CORS errors

The WASM files must be served from the same origin or with proper CORS headers.

### Memory access errors

Check that your plugins are using the correct ABI version and PDK headers.

### Plugin returns empty output

Make sure the plugin calls `pdk.Output()` or `pdk_output()` before returning.

### "Import #X module is not an object or function" error

This error occurs when a plugin tries to import a function from another module that hasn't been loaded yet.

**Solution**: Load modules in dependency order. If module A imports from module B, load B before A.

Example:
```javascript
// ❌ Wrong - greet imports from random but random is loaded after
modules: [
  { name: 'greet', url: './greet.wasm' },
  { name: 'random', url: './random.wasm' }
]

// ✅ Correct - random is loaded first
modules: [
  { name: 'random', url: './random.wasm' },
  { name: 'greet', url: './greet.wasm' }
]
```

**How to identify dependencies:**
- Look for `//go:wasmimport moduleName functionName` in Go plugins
- Check for `__attribute__((import_module("moduleName")))` in C plugins
- Use `wasm-objdump -x plugin.wasm` to see all imports

## Performance Tips

1. **Reuse the PluginManager**: Create once, call many times
2. **Use Uint8Array**: Avoid string encoding/decoding in hot paths
3. **Minimize plugin-to-plugin calls**: Each call has overhead
4. **Consider primitive host functions**: Zero-copy for numeric operations
5. **Batch operations**: Call plugins with bulk data when possible

## Examples

See the `example.html` file for a complete interactive demo showing:
- Loading plugins from URLs and files
- Calling plugin functions
- Native host functions
- Plugin-to-plugin communication
- Error handling

## License

MIT

## Contributing

Contributions are welcome! This SDK is part of the WASM Plugin project.

## See Also

- [Go SDK](../sdk/) - Server-side plugin manager
- [Go PDK](../pdk/pdk.go) - Plugin development kit for Go/TinyGo
- [C PDK](../pdk/pdk.h) - Plugin development kit for C
- [ABI Specification](../README.md) - Plugin ABI documentation

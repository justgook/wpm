# Logger Plugin (C Implementation)

This is a C implementation of the logger plugin, demonstrating the C PDK.

## Features

- ✅ Reads input using `pdk_input()`
- ✅ String manipulation (concatenation)
- ✅ Calls host functions using `pdk_call_host_str()`
- ✅ Sets output using `pdk_set_output()`
- ✅ No libc dependency (uses custom `pdk_memcpy` and `pdk_strlen`)

## Building

### Prerequisites

You need a C compiler with WebAssembly support. Options:

#### Option 1: WASI SDK (Recommended)

Download and install WASI SDK:
- [Releases](https://github.com/WebAssembly/wasi-sdk/releases)
- Install to `/opt/wasi-sdk` or set `WASI_SDK_PATH` environment variable

```bash
# Download WASI SDK (example for macOS ARM64)
curl -LO https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-27/wasi-sdk-27.0-arm64-macos.tar.gz
tar xzf wasi-sdk-27.0-arm64-macos.tar.gz
sudo mv wasi-sdk-27.0-arm64-macos /opt/wasi-sdk
```

#### Option 2: Clang with WebAssembly Support

Some system clang installations include WebAssembly targets. Check with:

```bash
clang --print-targets | grep -i wasm
```

### Build Commands

```bash
# Using make
make

# Clean build artifacts
make clean

# Or build manually with WASI SDK
/opt/wasi-sdk/bin/clang \
    -O3 -Wall \
    -Wl,--no-entry -Wl,--export=log \
    -o logger_c.wasm main.c

# Or with system clang (if it supports wasm32-wasi)
clang --target=wasm32-wasi \
    -nostdlib -O3 -Wall \
    -Wl,--no-entry -Wl,--export=log \
    -o logger_c.wasm main.c
```

## Code Structure

```c
#include "../../../pdk/pdk.h"

__attribute__((export_name("log")))
uint32_t log_message(void) {
    // 1. Get input
    uint32_t input_len;
    const uint8_t *input = pdk_input(&input_len);
    
    // 2. Process (add "[LOG] " prefix)
    const char prefix[] = "[LOG] ";
    uint32_t output_len = 6 + input_len;
    uint32_t output_ptr = pdk_alloc(output_len);
    uint8_t *output = (uint8_t*)output_ptr;
    
    pdk_memcpy(output, prefix, 6);
    pdk_memcpy(output + 6, input, input_len);
    
    // 3. Call host.print
    pdk_call_result_t result = pdk_call_host_str("print", output, output_len);
    
    // 4. Set output
    pdk_set_output(output_ptr, output_len);
    return (result.error != 0) ? 1 : 0;
}
```

## PDK Features Used

### Core ABI
- `pdk_alloc()` - Allocate memory
- `pdk_input_ptr()` / `pdk_input_len()` - Get input (via `pdk_input()`)
- `pdk_set_output()` - Set output buffer

### High-Level Helpers
- `pdk_input()` - Get input as pointer + length
- `pdk_call_host_str()` - Call host function with string name

### Utilities
- `pdk_memcpy()` - Copy memory (no libc)
- `pdk_strlen()` - String length (no libc)

## Comparison with Go Implementation

### Go (`plugins/logger/main.go`)
```go
func Log() uint32 {
    input := pdk.Input()
    message := string(input)
    output := "[LOG] " + message
    _, _, err := pdk.Call("host", "print", []byte(output))
    pdk.Output([]byte(output))
    return 0
}
```

### C (`plugins/logger_c/main.c`)
```c
uint32_t log_message(void) {
    uint32_t input_len;
    const uint8_t *input = pdk_input(&input_len);
    
    const char prefix[] = "[LOG] ";
    uint32_t output_len = 6 + input_len;
    uint32_t output_ptr = pdk_alloc(output_len);
    uint8_t *output = (uint8_t*)output_ptr;
    
    pdk_memcpy(output, prefix, 6);
    pdk_memcpy(output + 6, input, input_len);
    
    pdk_call_result_t result = pdk_call_host_str("print", output, output_len);
    pdk_set_output(output_ptr, output_len);
    return (result.error != 0) ? 1 : 0;
}
```

**Key Differences:**
- C requires manual memory management (`pdk_alloc`, `pdk_memcpy`)
- C has explicit length tracking (no automatic string/slice handling)
- C provides both string and length-based API variants
- C has zero libc dependencies

**Similarities:**
- Same ABI contract
- Same calling conventions
- Same input/output flow
- Interoperable (Go and C plugins can call each other)

## Testing

The C logger plugin is functionally identical to the Go logger plugin and can be tested the same way:

```bash
cd ../../
./build.sh
./run.sh
```

The host will call either `logger.wasm` (Go) or `logger_c.wasm` (C) with the same results.

#ifndef PDK_H
#define PDK_H

#include <stdint.h>

#ifndef NULL
#define NULL ((void *)0)
#endif

/* ============================================================================
 * WASM Plugin Development Kit (PDK) for C
 *
 * Single-header library for building WASM plugins in C.
 * Matches the Go PDK API for cross-language compatibility.
 *
 * Usage:
 *   #include "pdk.h"
 *
 *   __attribute__((export_name("your_function")))
 *   uint32_t your_function(void) {
 *       uint32_t len;
 *       const uint8_t *input = pdk_input(&len);
 *       // ... process input ...
 *       pdk_output(output_data, output_len);
 *       return 0;
 *   }
 *
 * Build:
 *   clang --target=wasm32-wasi -nostdlib -O3 \
 *         -Wl,--no-entry -Wl,--export=your_function \
 *         -o plugin.wasm main.c
 * ============================================================================
 */

/* ============================================================================
 * 1. INTERNAL UTILITIES (no libc dependency)
 * ============================================================================
 */

static inline uint32_t pdk_strlen(const char *str) {
  uint32_t len = 0;
  while (str[len] != '\0')
    len++;
  return len;
}

static inline void pdk_memcpy(void *dest, const void *src, uint32_t n) {
  uint8_t *d = (uint8_t *)dest;
  const uint8_t *s = (const uint8_t *)src;
  for (uint32_t i = 0; i < n; i++) {
    d[i] = s[i];
  }
}

/* ============================================================================
 * 2. CORE ABI - WASM IMPORTS FROM HOST
 *
 * These functions are provided by the host runtime and imported into the
 * WASM module. They form the minimal stable ABI between host and plugin.
 * ============================================================================
 */

/* Memory management */

// Allocate memory in host-managed space
// Returns: pointer (offset) to allocated memory
__attribute__((import_module("env"), import_name("alloc"))) uint32_t
pdk_alloc(uint64_t size);

// Free previously allocated memory
__attribute__((import_module("env"), import_name("free"))) void
pdk_free(uint32_t ptr);

/* Input/Output */

// Get pointer to input buffer provided by caller
__attribute__((import_module("env"), import_name("input_ptr"))) uint32_t
pdk_input_ptr(void);

// Get length of input buffer
__attribute__((import_module("env"), import_name("input_len"))) uint32_t
pdk_input_len(void);

// Set output buffer location for caller to read
__attribute__((import_module("env"), import_name("set_output"))) void
pdk_set_output(uint32_t ptr, uint32_t len);

/* Plugin-to-plugin communication */

// Call another plugin or host function
// Returns: 0 on success, non-zero error code on failure
__attribute__((import_module("env"), import_name("plugin_call"))) uint32_t
pdk_plugin_call(uint32_t module_ptr, uint32_t module_len, uint32_t func_ptr,
                uint32_t func_len, uint32_t input_ptr, uint32_t input_len);

// Get return value from last plugin_call
__attribute__((import_module("env"), import_name("plugin_call_return"))) int32_t
pdk_plugin_call_return(void);

// Get pointer to output from last plugin_call
__attribute__((import_module("env"),
               import_name("plugin_call_output_ptr"))) uint32_t
pdk_plugin_call_output_ptr(void);

// Get length of output from last plugin_call
__attribute__((import_module("env"),
               import_name("plugin_call_output_len"))) uint32_t
pdk_plugin_call_output_len(void);

/* ============================================================================
 * 3. HIGH-LEVEL INPUT/OUTPUT HELPERS
 *
 * Convenience functions that wrap the core ABI for easier usage.
 * ============================================================================
 */

// Get input data from caller
// Returns: pointer to input buffer (valid for duration of call)
// out_len: optional, will be set to input length if non-NULL
static inline const uint8_t *pdk_input(uint32_t *out_len) {
  uint32_t ptr = pdk_input_ptr();
  uint32_t len = pdk_input_len();
  if (out_len)
    *out_len = len;
  return (const uint8_t *)ptr;
}

// Set output data for caller
// Allocates new buffer, copies data, and registers with host
// The allocated memory is managed by the host after this call
static inline void pdk_output(const uint8_t *data, uint32_t len) {
  uint32_t out_ptr = pdk_alloc(len);
  uint8_t *out_buf = (uint8_t *)out_ptr;
  pdk_memcpy(out_buf, data, len);
  pdk_set_output(out_ptr, len);
}

/* ============================================================================
 * 4. PLUGIN CALL INFRASTRUCTURE
 *
 * High-level API for calling other plugins or host functions.
 * Provides both explicit-length and string-based variants.
 * ============================================================================
 */

// Result structure for plugin calls
// NOTE: output pointer references WASM linear memory and is only valid
// during the current execution context. Do not store for later use.
typedef struct {
  int32_t return_code;   // Return value from called function
  const uint8_t *output; // Pointer to output data (WASM linear memory)
  uint32_t output_len;   // Length of output data in bytes
  int32_t error;         // 0 = success, non-zero = error code from call
} pdk_call_result_t;

// Call another plugin or host function with explicit lengths
//
// Parameters:
//   module:     Name of the module to call (e.g., "logger", "host")
//   module_len: Length of module name
//   function:   Name of the function to call (e.g., "log", "print")
//   func_len:   Length of function name
//   input:      Input data to pass (can be NULL if input_len is 0)
//   input_len:  Length of input data
//
// Returns: pdk_call_result_t with result data
//   - On success: error = 0, return_code and output/output_len are set
//   - On failure: error = non-zero error code
static inline pdk_call_result_t
pdk_call(const char *module, uint32_t module_len, const char *function,
         uint32_t func_len, const uint8_t *input, uint32_t input_len) {
  pdk_call_result_t result = {0};

  // 1. Allocate and write module name
  uint32_t module_ptr = pdk_alloc(module_len);
  pdk_memcpy((void *)module_ptr, module, module_len);

  // 2. Allocate and write function name
  uint32_t func_ptr = pdk_alloc(func_len);
  pdk_memcpy((void *)func_ptr, function, func_len);

  // 3. Allocate and write input (if provided)
  uint32_t input_ptr = 0;
  if (input_len > 0 && input != NULL) {
    input_ptr = pdk_alloc(input_len);
    pdk_memcpy((void *)input_ptr, input, input_len);
  }

  // 4. Make the call
  uint32_t call_result = pdk_plugin_call(module_ptr, module_len, func_ptr,
                                         func_len, input_ptr, input_len);

  // 5. Free allocated memory
  pdk_free(module_ptr);
  pdk_free(func_ptr);
  if (input_ptr != 0) {
    pdk_free(input_ptr);
  }

  // 6. Check for errors
  if (call_result != 0) {
    result.error = (int32_t)call_result;
    return result;
  }

  // 7. Get return value and output (pointers to WASM linear memory)
  result.return_code = pdk_plugin_call_return();
  result.output = (const uint8_t *)pdk_plugin_call_output_ptr();
  result.output_len = pdk_plugin_call_output_len();
  result.error = 0;

  return result;
}

// Call with C strings (uses strlen internally)
static inline pdk_call_result_t pdk_call_str(const char *module,
                                             const char *function,
                                             const uint8_t *input,
                                             uint32_t input_len) {
  return pdk_call(module, pdk_strlen(module), function, pdk_strlen(function),
                  input, input_len);
}

// Call another plugin (convenience wrapper)
static inline pdk_call_result_t
pdk_call_plugin(const char *plugin, uint32_t plugin_len, const char *function,
                uint32_t func_len, const uint8_t *input, uint32_t input_len) {
  return pdk_call(plugin, plugin_len, function, func_len, input, input_len);
}

// Call another plugin with C strings
static inline pdk_call_result_t pdk_call_plugin_str(const char *plugin,
                                                    const char *function,
                                                    const uint8_t *input,
                                                    uint32_t input_len) {
  return pdk_call_str(plugin, function, input, input_len);
}

// Call host function (convenience wrapper)
static inline pdk_call_result_t pdk_call_host(const char *function,
                                              uint32_t func_len,
                                              const uint8_t *input,
                                              uint32_t input_len) {
  return pdk_call("host", 4, function, func_len, input, input_len);
}

// Call host function with C string
static inline pdk_call_result_t pdk_call_host_str(const char *function,
                                                  const uint8_t *input,
                                                  uint32_t input_len) {
  return pdk_call("host", 4, function, pdk_strlen(function), input, input_len);
}

#endif // PDK_H

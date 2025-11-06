#include "../../../pdk/pdk.h"

// Logger plugin implemented in C
// Demonstrates:
// - Reading input
// - String manipulation
// - Calling host functions
// - Setting output

__attribute__((export_name("log"))) uint32_t log_message(void) {
  // 1. Get input from caller
  uint32_t input_len;
  const uint8_t *input = pdk_input(&input_len);

  // 2. Format message: "[LOG] " + input
  const char prefix[] = "[LOG] ";
  const uint32_t prefix_len = 6;
  uint32_t output_len = prefix_len + input_len;

  // Allocate output buffer
  uint32_t output_ptr = pdk_alloc(output_len);
  uint8_t *output = (uint8_t *)output_ptr;

  // Copy prefix
  pdk_memcpy(output, prefix, prefix_len);

  // Copy input message
  pdk_memcpy(output + prefix_len, input, input_len);

  // 3. Call host.print to output to console
  pdk_call_result_t result = pdk_call_host_str("print", output, output_len);

  if (result.error != 0) {
    // If host call fails, still output the formatted message
    pdk_set_output(output_ptr, output_len);
    return 1;
  }

  // 4. Return formatted log message to caller
  pdk_set_output(output_ptr, output_len);
  return 0;
}

# Makefile for C WASM plugin
#
# Requires either:
# 1. WASI SDK: https://github.com/WebAssembly/wasi-sdk/releases
# 2. Clang with WebAssembly support
#
# Set WASI_SDK_PATH to use WASI SDK, otherwise uses system clang

# Try to find WASI SDK
WASI_SDK_PATH ?= /opt/wasi-sdk
ifneq ("$(wildcard $(WASI_SDK_PATH)/bin/clang)","")
    CC = $(WASI_SDK_PATH)/bin/clang
    CFLAGS = -O3 -Wall -nostartfiles
    LDFLAGS = -Wl,--no-entry -Wl,--strip-all
else
    # Fall back to system clang with wasm32 target
    CC = clang
    CFLAGS = --target=wasm32-wasi -nostdlib -O3 -Wall
    LDFLAGS = -Wl,--no-entry -Wl,--export=log
endif

TARGET = logger_c.wasm
SRC = main.c
OUTPUT_DIR = ../../cmd/app

all: $(TARGET)

$(TARGET): $(SRC)
	@echo "Building with: $(CC)"
	$(CC) $(CFLAGS) $(LDFLAGS) -o $(TARGET) $(SRC)
	@mkdir -p $(OUTPUT_DIR)
	mv $(TARGET) $(OUTPUT_DIR)/

clean:
	rm -f $(TARGET) $(OUTPUT_DIR)/$(TARGET)

.PHONY: all clean

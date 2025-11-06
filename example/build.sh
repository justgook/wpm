#!/usr/bin/env bash

set -e

echo "========================================"
echo "Building WASM Plugins"
echo "========================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

BUILD_DIR="cmd/app"
mkdir -p "$BUILD_DIR"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Build Go plugins with TinyGo
echo -e "${BLUE}Building Go plugins with TinyGo...${NC}"
if command_exists tinygo; then
    echo "  → host.wasm"
    GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o "$BUILD_DIR/host.wasm" ./plugins/host/main.go
    
    echo "  → logger.wasm"
    GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o "$BUILD_DIR/logger.wasm" ./plugins/logger/main.go
    
    echo "  → greet.wasm"
    GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o "$BUILD_DIR/greet.wasm" ./plugins/greet/main.go
    
    echo "  → mathtest.wasm"
    GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o "$BUILD_DIR/mathtest.wasm" ./plugins/mathtest/main.go
    
    echo -e "${GREEN}✓ Go plugins built successfully${NC}"
else
    echo -e "${RED}✗ TinyGo not found. Skipping Go plugins.${NC}"
    echo "  Install from: https://tinygo.org/getting-started/install/"
fi
echo ""

# Build Zig plugin
echo -e "${BLUE}Building Zig plugin...${NC}"
if command_exists zig; then
    echo "  → random.wasm"
    zig build-exe ./plugins/random/main.zig \
        -target wasm32-freestanding \
        -fno-entry \
        -rdynamic \
        -O ReleaseFast \
        -femit-bin="$BUILD_DIR/random.wasm"
    
    echo -e "${GREEN}✓ Zig plugin built successfully${NC}"
else
    echo -e "${RED}✗ Zig not found. Skipping Zig plugin.${NC}"
    echo "  Install from: https://ziglang.org/download/"
fi
echo ""

# Build C plugin
echo -e "${BLUE}Building C plugin...${NC}"
if [ -x "/opt/wasi-sdk/bin/clang" ]; then
    echo "  → logger_c.wasm (using WASI SDK)"
    cd plugins/logger_c
    make clean >/dev/null 2>&1
    if make; then
        cd ../..
        echo -e "${GREEN}✓ C plugin built successfully${NC}"
    else
        cd ../..
        echo -e "${RED}✗ C plugin build failed${NC}"
    fi
elif command_exists clang; then
    echo "  → logger_c.wasm (using system clang)"
    cd plugins/logger_c
    make clean >/dev/null 2>&1
    if make 2>/dev/null; then
        cd ../..
        echo -e "${GREEN}✓ C plugin built successfully${NC}"
    else
        cd ../..
        echo -e "${RED}✗ System clang lacks WebAssembly support. Skipping C plugin.${NC}"
        echo "  Install WASI SDK from: https://github.com/WebAssembly/wasi-sdk/releases"
        echo "  or use a clang version with WebAssembly target support"
    fi
else
    echo -e "${RED}✗ Clang not found. Skipping C plugin.${NC}"
    echo "  Install WASI SDK from: https://github.com/WebAssembly/wasi-sdk/releases"
fi
echo ""

# List built files
echo "========================================"
echo "Built WASM modules:"
echo "========================================"
ls -lh "$BUILD_DIR"/*.wasm 2>/dev/null || echo "No WASM files found"
echo ""

echo -e "${GREEN}Build complete!${NC}"
echo "Run './run.sh' to execute the example"

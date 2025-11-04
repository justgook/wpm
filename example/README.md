# Example Build Instructions

This directory contains example plugins and the CLI application for the WASM plugin system.

## Prerequisites

- Go 1.19+
- TinyGo (for building Go plugins)
- Zig (for building Zig plugins)

## Build Sequence

1. **Build the CLI application:**
   ```
   go build -o app ./cmd/app/
   ```

2. **Build plugins:**

   For Go plugins (e.g., greet plugin):
   ```
   GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o greet.wasm ./plugins/greet/main.go
   ```

    For Zig plugins (e.g., random plugin):
    ```
    zig build-exe ./plugins/random/main.zig -target wasm32-freestanding -fno-entry -rdynamic -O ReleaseFast -femit-bin=random.wasm
    ```

3. **Run the CLI with plugins:**
   ```
   go run ./cmd/app/
   ```

The current directory will contain the built CLI binary (`app`) and WASM plugin files (`greet.wasm`, `random.wasm`).
#!/usr/bin/env bash

set -e

echo "========================================"
echo "WASM Plugin System - Build & Run"
echo "========================================"
echo ""

# Build all plugins
./build.sh

# Run the example
echo ""
echo "========================================"
echo "Running Example Application"
echo "========================================"
echo ""

cd cmd/app
go run main.go

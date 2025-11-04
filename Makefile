SHELL := bash
.ONESHELL:
.SECONDEXPANSION:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

ifdef V
Q=
WGET:=wget
else
Q=@
MAKEFLAGS += --no-print-directory
WGET:=wget -q --show-progress
endif

uniq = $(if $1,$(firstword $1) $(call uniq,$(filter-out $(firstword $1),$1)))

define QUIET
	$(if $(V), , $(1))
endef

MKDIR_P ?= mkdir -p
CP ?= cp -f

.DEFAULT_GOAL := all


BUILD_DIR ?= build.nosync
PLUGIN_DIR ?= plugins

# Detect all plugin subdirectories
PLUGIN_DIRS := $(wildcard $(PLUGIN_DIR)/*)
PLUGINS := $(notdir $(PLUGIN_DIRS))
PLUGIN_TARGETS := $(addprefix $(BUILD_DIR)/,$(addsuffix .wasm,$(PLUGINS)))


.PHONY: all
all: cli 


.PHONY: plugins-release
plugins-release: $(PLUGIN_TARGETS)

# Rule to build Go plugins
$(BUILD_DIR)/%.wasm: $(PLUGIN_DIR)/%/main.go $(wildcard $(PLUGIN_DIR)/%/*.go) | $(BUILD_DIR)
	$(Q)echo "Building Go plugin $*..."
	# $(Q)tinygo build -o $@ -target wasip1 -buildmode=c-shared $<
	$(Q)GOOS=wasip1 GOARCH=wasm tinygo build -buildmode=c-shared -o $@ $<


# Rule to build Zig plugins
$(BUILD_DIR)/%.wasm: $(PLUGIN_DIR)/%/main.zig $(wildcard $(PLUGIN_DIR)/%/*.zig) | $(BUILD_DIR)
	$(Q)echo "Building Zig plugin $*..."
	$(Q)zig build-exe $< -target wasm32-freestanding -fno-entry -rdynamic -O ReleaseFast -femit-bin=$@


.PHONY: clean
clean:
	$(Q)git ls-files -oi --exclude-standard | (grep -v '^\.idea' || exit 0) | xargs trash
	$(Q)rm -rf $(BUILD_DIR)

.PHONY: web
web:
	$(Q) echo "building web"

.PHONY: release-win64
release-win64:
	$(Q) echo "building release-win64"

.PHONY: release-mac
release-mac:
	$(Q) echo "building release-mac"

.PHONY: release-linux
release-linux:
	$(Q) echo "building release-kinux"

.PHONY: lsp 
lsp:
	$(Q)go build -o $(BUILD_DIR)/lsp ./cmd/lsp/

.PHONY: cli 
cli:
	$(Q)go build -o $(BUILD_DIR)/cli ./cmd/cli/

.PHONY: cli-run
cli-run: $(PLUGIN_TARGETS)
	$(Q)go run ./cmd/cli/

# Ensure build directory exists
$(BUILD_DIR):
	$(Q)mkdir -p $@

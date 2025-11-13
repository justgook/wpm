/**
 * WASM Plugin SDK for JavaScript (Browser)
 * 
 * This SDK allows you to load and manage WebAssembly plugins in the browser,
 * compatible with plugins built using the Go PDK (pdk/pdk.go) or C PDK (pdk/pdk.h).
 * 
 * Usage:
 *   const manager = await PluginManager.create({
 *     modules: [
 *       { name: 'logger', url: '/plugins/logger.wasm' },
 *       { name: 'greet', data: wasmBytesArrayBuffer }
 *     ],
 *     hostFunctions: [
 *       {
 *         module: 'host',
 *         function: 'print',
 *         handler: (input) => {
 *           console.log(new TextDecoder().decode(input));
 *           return { returnCode: 0, output: new Uint8Array() };
 *         }
 *       }
 *     ]
 *   });
 *   
 *   const { returnCode, output } = await manager.call('greet', 'greet', 'World');
 */

class PluginManager {
  constructor() {
    this.wasmModules = new Map();
    this.hostModules = new Map();
    this.hostFunctionDefs = new Map();
    this.memoryOffsets = new Map();
    this.config = {
      envModuleName: 'env',
      maxCallDepth: 10
    };

    // Current call state
    this.currentInputPtr = 0;
    this.currentInputLen = 0;
    this.currentOutputPtr = 0;
    this.currentOutputLen = 0;

    // Plugin-to-plugin call state
    this.callStack = [];
    this.lastCallReturn = 0;
    this.lastCallOutputPtr = 0;
    this.lastCallOutputLen = 0;

    // Text encoding/decoding
    this.textEncoder = new TextEncoder();
    this.textDecoder = new TextDecoder();
  }

  /**
   * Create a new PluginManager instance
   * @param {Object} options
   * @param {Array<{name: string, url?: string, data?: ArrayBuffer}>} options.modules
   * @param {Array<{module: string, function: string, handler: Function}>} options.hostFunctions
   * @param {Object} options.config
   * @returns {Promise<PluginManager>}
   */
  static async create(options = {}) {
    const manager = new PluginManager();

    if (options.config) {
      Object.assign(manager.config, options.config);
    }

    // Setup host functions first
    await manager.setupHostFunctions(options.hostFunctions || []);

    // Load WASM modules
    for (const module of options.modules || []) {
      await manager.loadWasmModule(module);
    }

    return manager;
  }

  /**
   * Setup host functions (both native JS and WASM imports)
   */
  async setupHostFunctions(hostFunctions) {
    // Group by module name
    const hostModuleGroups = new Map();

    for (const fn of hostFunctions) {
      if (!hostModuleGroups.has(fn.module)) {
        hostModuleGroups.set(fn.module, []);
        this.hostFunctionDefs.set(fn.module, new Map());
      }
      hostModuleGroups.get(fn.module).push(fn);
      this.hostFunctionDefs.get(fn.module).set(fn.function, fn);
    }

    // Always create the env module
    if (!hostModuleGroups.has(this.config.envModuleName)) {
      hostModuleGroups.set(this.config.envModuleName, []);
    }

    // Store host modules (we don't instantiate them in JS like Go does)
    for (const [moduleName, functions] of hostModuleGroups) {
      this.hostModules.set(moduleName, { name: moduleName, functions });
    }
  }

  /**
   * Load a WASM module
   * @param {Object} module
   * @param {string} module.name
   * @param {string} [module.url]
   * @param {ArrayBuffer} [module.data]
   */
  async loadWasmModule(module) {
    let wasmBytes;

    if (module.data) {
      wasmBytes = module.data;
    } else if (module.url) {
      const response = await fetch(module.url);
      wasmBytes = await response.arrayBuffer();
    } else {
      throw new Error(`Module ${module.name} must have either url or data`);
    }

    // Create import object with env functions
    const importObject = this.createImportObject(module.name);

    // Instantiate the WASM module
    const wasmModule = await WebAssembly.instantiate(wasmBytes, importObject);

    // Initialize if _initialize exists (TinyGo compatibility)
    if (wasmModule.instance.exports._initialize) {
      wasmModule.instance.exports._initialize();
    }

    this.wasmModules.set(module.name, {
      name: module.name,
      instance: wasmModule.instance,
      memory: wasmModule.instance.exports.memory
    });

    // Pre-allocate a small amount of memory to ensure the memory system is initialized
    // This prevents issues with the first cross-plugin call
    try {
      const initPtr = this.allocFunc(module.name, 64); // Allocate 64 bytes for initialization
      if (initPtr > 0) {
        // Successfully initialized memory allocation system
        console.log(`Memory system initialized for plugin: ${module.name}`);
      }
    } catch (e) {
      console.warn(`Failed to pre-initialize memory for ${module.name}:`, e);
    }
  }

  /**
   * Create import object for WASM instantiation
   */
  createImportObject(moduleName) {
    const importObject = {};

    // Add env module (always present)
    importObject[this.config.envModuleName] = {
      alloc: (size) => this.allocFunc(moduleName, Number(size)),
      free: (ptr) => this.freeFunc(moduleName, ptr),
      input_ptr: () => this.inputPtrFunc(),
      input_len: () => this.inputLenFunc(),
      set_output: (ptr, len) => this.setOutputFunc(ptr, len),

      // Plugin-to-plugin call functions
      plugin_call: (modulePtr, moduleLen, funcPtr, funcLen, inputPtr, inputLen) =>
        this.pluginCallFunc(moduleName, modulePtr, moduleLen, funcPtr, funcLen, inputPtr, inputLen),
      plugin_call_return: () => this.pluginCallReturnFunc(),
      plugin_call_output_ptr: () => this.pluginCallOutputPtrFunc(),
      plugin_call_output_len: () => this.pluginCallOutputLenFunc()
    };

    // Add WASI support (minimal polyfill for TinyGo/WASI plugins)
    importObject.wasi_snapshot_preview1 = {
      // File descriptor operations (no-ops for browser)
      fd_close: () => 0,
      fd_write: (fd, _iovs, _iovsLen, _nwritten) => {
        // Minimal console.log support for stdout/stderr
        if (fd === 1 || fd === 2) {
          // fd 1 = stdout, fd 2 = stderr
          // For now, just return success
          return 0;
        }
        return 0;
      },
      fd_read: () => 0,
      fd_seek: () => 0,
      fd_fdstat_get: () => 0,
      fd_fdstat_set_flags: () => 0,
      fd_prestat_get: () => 8, // Return EBADF (bad file descriptor)
      fd_prestat_dir_name: () => 0,

      // Path operations (no-ops)
      path_open: () => 8,
      path_filestat_get: () => 8,
      path_remove_directory: () => 8,
      path_unlink_file: () => 8,

      // Environment
      environ_sizes_get: (environCount, environBufSize) => {
        // No environment variables
        return 0;
      },
      environ_get: () => 0,

      // Arguments
      args_sizes_get: (argc, argvBufSize) => {
        // No command line arguments
        return 0;
      },
      args_get: () => 0,

      // Clock
      clock_time_get: (clockId, precision, timestamp) => {
        // Return current time in nanoseconds
        return 0;
      },

      // Random
      random_get: (buf, bufLen) => {
        // Fill with random bytes using crypto API if available
        if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
          const module = this.wasmModules.get(moduleName);
          if (module) {
            const memory = new Uint8Array(module.memory.buffer);
            const randomBytes = new Uint8Array(bufLen);
            crypto.getRandomValues(randomBytes);
            memory.set(randomBytes, buf);
          }
        }
        return 0;
      },

      // Process
      proc_exit: (code) => {
        throw new Error(`WASI proc_exit called with code ${code}`);
      },

      // Poll (no-op)
      poll_oneoff: () => 0,

      // Socket operations (no-ops)
      sock_recv: () => 8,
      sock_send: () => 8,
      sock_shutdown: () => 8
    };

    // Add exports from already-loaded WASM modules (for cross-module imports)
    for (const [wasmModuleName, wasmModule] of this.wasmModules) {
      // Skip if it's the module we're currently loading
      if (wasmModuleName === moduleName) continue;

      importObject[wasmModuleName] = {};

      // Export all functions from this module
      const exports = wasmModule.instance.exports;
      for (const [exportName, exportValue] of Object.entries(exports)) {
        // Only export functions, not memory or other exports
        if (typeof exportValue === 'function') {
          importObject[wasmModuleName][exportName] = exportValue;
        }
      }
    }

    // Add other host modules
    for (const [hostModuleName, hostModule] of this.hostModules) {
      if (hostModuleName === this.config.envModuleName) continue;

      importObject[hostModuleName] = {};
      const functionMap = this.hostFunctionDefs.get(hostModuleName);

      if (functionMap) {
        for (const [funcName, funcDef] of functionMap) {
          importObject[hostModuleName][funcName] = (...args) => {
            return this.callHostFunctionFromWasm(moduleName, funcDef, args);
          };
        }
      }
    }

    return importObject;
  }

  /**
   * Allocate memory in a module's linear memory
   * Strategy: Always allocate at the END of current memory and grow if needed
   * This avoids conflicts with TinyGo's heap which grows upward from data segments
   */
  allocFunc(moduleName, size) {
    const module = this.wasmModules.get(moduleName);
    if (!module) return 0;

    // Ensure minimum size for allocation
    if (size === 0) size = 1;

    // Calculate how many pages we need to add for this allocation
    // We'll add extra pages to reduce frequency of grows
    const pagesNeeded = Math.ceil(size / 65536) || 1;

    // Grow memory with proper error handling and initialization
    try {
      const oldPages = module.memory.grow(pagesNeeded);
      const allocPtr = oldPages * 65536;

      // Initialize the allocated memory to zeros to avoid garbage data issues
      const memory = new Uint8Array(module.memory.buffer);
      for (let i = allocPtr; i < allocPtr + size; i++) {
        memory[i] = 0;
      }

      return allocPtr;
    } catch (e) {
      console.error(`Failed to grow memory for ${moduleName}: tried to add ${pagesNeeded} pages (${size} bytes requested)`, e);
      throw new Error(`Out of memory in ${moduleName}`);
    }
  }

  freeFunc(moduleName, ptr) {
    // No-op - we can't shrink WASM memory
    // The memory will be reused after the WASM instance is recreated
  }

  inputPtrFunc() {
    return this.currentInputPtr;
  }

  inputLenFunc() {
    return this.currentInputLen;
  }

  setOutputFunc(ptr, len) {
    this.currentOutputPtr = ptr;
    this.currentOutputLen = len;
  }

  /**
   * Plugin-to-plugin call implementation
   */
  pluginCallFunc(callerModuleName, modulePtr, moduleLen, funcPtr, funcLen, inputPtr, inputLen) {
    try {
      // Get caller's memory
      const callerModule = this.wasmModules.get(callerModuleName);
      if (!callerModule) return 1;

      const memory = new Uint8Array(callerModule.memory.buffer);

      // Read module and function names
      const moduleName = this.textDecoder.decode(
        memory.slice(modulePtr, modulePtr + moduleLen)
      );
      const funcName = this.textDecoder.decode(
        memory.slice(funcPtr, funcPtr + funcLen)
      );

      // Read input
      const input = inputLen > 0
        ? memory.slice(inputPtr, inputPtr + inputLen)
        : new Uint8Array(0);

      // Check call depth
      if (this.callStack.length >= this.config.maxCallDepth) {
        return 4; // max depth exceeded
      }

      // Save current context
      const currentCtx = {
        moduleName: callerModuleName,
        inputPtr: this.currentInputPtr,
        inputLen: this.currentInputLen,
        outputPtr: this.currentOutputPtr,
        outputLen: this.currentOutputLen
      };
      this.callStack.push(currentCtx);

      // Make the call (synchronous)
      const result = this.callSync(moduleName, funcName, input);

      // Restore context
      this.callStack.pop();
      this.currentInputPtr = currentCtx.inputPtr;
      this.currentInputLen = currentCtx.inputLen;
      this.currentOutputPtr = currentCtx.outputPtr;
      this.currentOutputLen = currentCtx.outputLen;

      // Store results
      this.lastCallReturn = result.returnCode;

      if (result.output.length > 0) {
        // Allocate in caller's memory
        const outputPtr = this.allocFunc(callerModuleName, result.output.length);
        const callerMemory = new Uint8Array(callerModule.memory.buffer);
        callerMemory.set(result.output, outputPtr);

        this.lastCallOutputPtr = outputPtr;
        this.lastCallOutputLen = result.output.length;
      } else {
        this.lastCallOutputPtr = 0;
        this.lastCallOutputLen = 0;
      }

      return 0; // success
    } catch (error) {
      console.error('Plugin call failed:', error);
      return 5; // call failed
    }
  }

  pluginCallReturnFunc() {
    return this.lastCallReturn;
  }

  pluginCallOutputPtrFunc() {
    return this.lastCallOutputPtr;
  }

  pluginCallOutputLenFunc() {
    return this.lastCallOutputLen;
  }

  /**
   * Call a host function from WASM
   */
  callHostFunctionFromWasm(callerModuleName, funcDef, args) {
    // For ByteHandler-style functions
    if (funcDef.isByteHandler !== false) {
      const callerModule = this.wasmModules.get(callerModuleName);
      if (!callerModule) return 0;

      // Read input from current state
      let input = new Uint8Array(0);
      if (this.currentInputLen > 0) {
        const memory = new Uint8Array(callerModule.memory.buffer);
        input = memory.slice(this.currentInputPtr, this.currentInputPtr + this.currentInputLen);
      }

      // Call the handler
      const result = funcDef.handler(input);
      const returnCode = result.returnCode || 0;
      const output = result.output || new Uint8Array(0);

      // Write output back to caller's memory
      if (output.length > 0) {
        const outputPtr = this.allocFunc(callerModuleName, output.length);
        const memory = new Uint8Array(callerModule.memory.buffer);
        memory.set(output, outputPtr);
        this.currentOutputPtr = outputPtr;
        this.currentOutputLen = output.length;
      }

      return returnCode;
    } else {
      // For primitive functions, just call directly
      return funcDef.handler(...args);
    }
  }

  /**
   * Synchronous call (used internally by plugin_call)
   */
  callSync(moduleName, functionName, input) {
    // Try WASM modules first
    if (this.wasmModules.has(moduleName)) {
      return this.callWasmFunction(moduleName, functionName, input);
    }

    // Try host modules
    if (this.hostModules.has(moduleName)) {
      return this.callHostFunction(moduleName, functionName, input);
    }

    throw new Error(`Module ${moduleName} not found`);
  }

  /**
   * Call a WASM function
   */
  callWasmFunction(moduleName, functionName, input) {
    const module = this.wasmModules.get(moduleName);
    if (!module) {
      throw new Error(`WASM module ${moduleName} not found`);
    }

    try {
      // Write input to memory
      const inputPtr = this.allocFunc(moduleName, input.length);
      const currentMemory = new Uint8Array(module.memory.buffer); // Refresh in case memory grew
      currentMemory.set(input, inputPtr);
      this.currentInputPtr = inputPtr;
      this.currentInputLen = input.length;

      // Reset output
      this.currentOutputPtr = 0;
      this.currentOutputLen = 0;

      // Call the function
      const fn = module.instance.exports[functionName];
      if (!fn) {
        throw new Error(`Function ${functionName} not found in module ${moduleName}`);
      }

      const returnValue = fn() || 0;

      // Read output
      let output = new Uint8Array(0);
      if (this.currentOutputPtr !== 0 && this.currentOutputLen > 0) {
        const updatedMemory = new Uint8Array(module.memory.buffer);
        output = updatedMemory.slice(this.currentOutputPtr, this.currentOutputPtr + this.currentOutputLen);
      }

      return {
        returnCode: returnValue,
        output: output
      };
    } finally {
      // No cleanup needed - memory grows but data in TinyGo's heap is preserved
    }
  }

  /**
   * Call a host function
   */
  callHostFunction(moduleName, functionName, input) {
    const functionMap = this.hostFunctionDefs.get(moduleName);
    if (!functionMap) {
      throw new Error(`Host module ${moduleName} not found`);
    }

    const funcDef = functionMap.get(functionName);
    if (!funcDef) {
      throw new Error(`Function ${functionName} not found in host module ${moduleName}`);
    }

    // Call the handler
    const result = funcDef.handler(input);

    return {
      returnCode: result.returnCode || 0,
      output: result.output || new Uint8Array(0)
    };
  }

  /**
   * Public API: Call a function in any module
   * @param {string} moduleName
   * @param {string} functionName
   * @param {string|Uint8Array} input
   * @returns {Promise<{returnCode: number, output: Uint8Array}>}
   */
  async call(moduleName, functionName, input) {
    // Convert string input to Uint8Array
    if (typeof input === 'string') {
      input = this.textEncoder.encode(input);
    } else if (!(input instanceof Uint8Array)) {
      input = new Uint8Array(input);
    }

    return this.callSync(moduleName, functionName, input);
  }

  rawCall(moduleName, functionName, input) {
    const module = this.wasmModules.get(moduleName)
    if (!module) {
      throw new Error(`WASM module ${moduleName} not found`)
    }

    const fn = module.instance.exports[functionName]
    if (!fn) {
      throw new Error(`Function ${functionName} not found in module ${moduleName}`)
    }

    return fn(input)
  }

  /**
   * Close and cleanup
   */
  close() {
    this.wasmModules.clear();
    this.hostModules.clear();
    this.hostFunctionDefs.clear();
    this.memoryOffsets.clear();
  }
}

// Export for both browser and module environments
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { PluginManager };
} else if (typeof window !== 'undefined') {
  window.PluginManager = PluginManager;
}

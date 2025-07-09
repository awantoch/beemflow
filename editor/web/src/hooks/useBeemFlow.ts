import { useCallback, useEffect, useState } from 'react'

// Global declarations for WASM functions
declare global {
  function beemflowParseYaml(yaml: string): WasmResult
  function beemflowValidateYaml(yaml: string): WasmResult
  function beemflowGenerateMermaid(yaml: string): WasmResult
  function beemflowYamlToVisual(yaml: string): WasmResult
  function beemflowVisualToYaml(visual: any): WasmResult
  
  // Go WASM runtime
  class Go {
    importObject: WebAssembly.Imports
    run(instance: WebAssembly.Instance): Promise<void>
  }
}

interface WasmResult {
  success: boolean
  data?: any
  error?: string
}

export interface VisualData {
  nodes: any[]
  edges: any[]
  flow: any
}

// Configuration constants
const WASM_CONFIG = {
  TIMEOUT_MS: 30000,
  RUNTIME_SCRIPT: '/wasm_exec.js',
  WASM_MODULE: '/main.wasm',
} as const

export const useBeemFlow = () => {
  const [wasmLoaded, setWasmLoaded] = useState(false)
  const [wasmError, setWasmError] = useState<string | null>(null)

  // Common error handler
  const handleError = useCallback((operation: string, error: unknown) => {
    const errorMessage = `${operation} failed: ${error}`
    console.error(errorMessage)
    return errorMessage
  }, [])

  // Common WASM function wrapper
  const callWasmFunction = useCallback(<T>(
    fn: () => WasmResult,
    operation: string,
    defaultValue: T
  ): T => {
    if (!wasmLoaded) {
      console.warn(`${operation} called before WASM loaded`)
      return defaultValue
    }
    
    try {
      const result = fn()
      if (result.success) {
        return result.data as T
      } else {
        console.error(`${operation} error:`, result.error)
        return defaultValue
      }
    } catch (error) {
      handleError(operation, error)
      return defaultValue
    }
  }, [wasmLoaded, handleError])

  // Load WASM module
  useEffect(() => {
    const loadWasm = async () => {
      try {
        // Check WebAssembly support
        if (!WebAssembly) {
          throw new Error('WebAssembly not supported in this browser')
        }

        // Load WASM runtime script
        await loadWasmRuntime()
        
        // Initialize WASM module
        await initializeWasmModule()
        
        setWasmLoaded(true)
      } catch (error) {
        setWasmError(handleError('WASM initialization', error))
      }
    }

    loadWasm()
  }, [handleError])

  // Load WASM runtime script
  const loadWasmRuntime = useCallback((): Promise<void> => {
    return new Promise((resolve, reject) => {
      const script = document.createElement('script')
      script.src = WASM_CONFIG.RUNTIME_SCRIPT
      script.async = true
      
      script.onload = () => resolve()
      script.onerror = () => reject(new Error('Failed to load WASM runtime script'))
      
      document.head.appendChild(script)
    })
  }, [])

  // Initialize WASM module
  const initializeWasmModule = useCallback(async (): Promise<void> => {
    const go = new Go()
    
    const wasmPromise = WebAssembly.instantiateStreaming(
      fetch(WASM_CONFIG.WASM_MODULE),
      go.importObject
    )
    
    const timeoutPromise = new Promise<never>((_, reject) => 
      setTimeout(() => reject(new Error('WASM load timeout')), WASM_CONFIG.TIMEOUT_MS)
    )
    
    const result = await Promise.race([wasmPromise, timeoutPromise])
    
    // Run the Go program
    await go.run(result.instance)
  }, [])

  // WASM function wrappers
  const parseYaml = useCallback((yaml: string) => {
    return callWasmFunction(
      () => beemflowParseYaml(yaml),
      'Parse YAML',
      null
    )
  }, [callWasmFunction])

  const validateYaml = useCallback((yaml: string): WasmResult => {
    return callWasmFunction(
      () => beemflowValidateYaml(yaml),
      'Validate YAML',
      { success: false, error: 'WASM not loaded' }
    )
  }, [callWasmFunction])

  const generateMermaid = useCallback((yaml: string): string => {
    return callWasmFunction(
      () => beemflowGenerateMermaid(yaml),
      'Generate Mermaid',
      ''
    )
  }, [callWasmFunction])

  const yamlToVisual = useCallback((yaml: string): VisualData => {
    return callWasmFunction(
      () => beemflowYamlToVisual(yaml),
      'YAML to Visual',
      { nodes: [], edges: [], flow: null }
    )
  }, [callWasmFunction])

  const visualToYaml = useCallback((visual: VisualData): string => {
    return callWasmFunction(
      () => beemflowVisualToYaml(visual),
      'Visual to YAML',
      ''
    )
  }, [callWasmFunction])

  return {
    wasmLoaded,
    wasmError,
    parseYaml,
    validateYaml,
    generateMermaid,
    yamlToVisual,
    visualToYaml,
  }
}
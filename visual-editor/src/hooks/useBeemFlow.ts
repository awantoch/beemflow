import { useCallback, useEffect, useState } from 'react'

// Global declarations for WASM functions
declare global {
  function beemflowParseYaml(yaml: string): any
  function beemflowValidateYaml(yaml: string): any
  function beemflowGenerateMermaid(yaml: string): any
  function beemflowYamlToVisual(yaml: string): any
  function beemflowVisualToYaml(visual: any): any
  
  // Go WASM runtime
  class Go {
    importObject: WebAssembly.Imports
    run(instance: WebAssembly.Instance): Promise<void>
  }
}

export interface FlowValidation {
  valid: boolean
  message: string
}

export interface VisualData {
  nodes: any[]
  edges: any[]
  flow: any
}

export const useBeemFlow = () => {
  const [wasmLoaded, setWasmLoaded] = useState(false)
  const [wasmError, setWasmError] = useState<string | null>(null)

  useEffect(() => {
    const loadWasm = async () => {
      try {
        // Load the Go WASM runtime
        const script = document.createElement('script')
        script.src = '/wasm_exec.js'
        document.head.appendChild(script)

        script.onload = async () => {
          const go = new Go()
          
          // Load and instantiate the WASM module
          const result = await WebAssembly.instantiateStreaming(
            fetch('/main.wasm'),
            go.importObject
          )
          
          // Run the Go program
          go.run(result.instance)
          setWasmLoaded(true)
        }

        script.onerror = () => {
          setWasmError('Failed to load WASM runtime')
        }
      } catch (error) {
        setWasmError(`Failed to initialize WASM: ${error}`)
      }
    }

    loadWasm()
  }, [])

  const parseYaml = useCallback((yaml: string) => {
    if (!wasmLoaded) return null
    
    try {
      const result = beemflowParseYaml(yaml)
      return result.success ? result.data : null
    } catch (error) {
      console.error('Failed to parse YAML:', error)
      return null
    }
  }, [wasmLoaded])

  const validateYaml = useCallback((yaml: string): FlowValidation => {
    if (!wasmLoaded) return { valid: false, message: 'WASM not loaded' }
    
    try {
      return beemflowValidateYaml(yaml)
    } catch (error) {
      return { valid: false, message: `Validation error: ${error}` }
    }
  }, [wasmLoaded])

  const generateMermaid = useCallback((yaml: string): string => {
    if (!wasmLoaded) return ''
    
    try {
      const result = beemflowGenerateMermaid(yaml)
      return result.success ? result.data : ''
    } catch (error) {
      console.error('Failed to generate Mermaid:', error)
      return ''
    }
  }, [wasmLoaded])

  const yamlToVisual = useCallback((yaml: string): VisualData => {
    if (!wasmLoaded) return { nodes: [], edges: [], flow: null }
    
    try {
      const result = beemflowYamlToVisual(yaml)
      return result.success ? result.data : { nodes: [], edges: [], flow: null }
    } catch (error) {
      console.error('Failed to convert YAML to visual:', error)
      return { nodes: [], edges: [], flow: null }
    }
  }, [wasmLoaded])

  const visualToYaml = useCallback((visual: VisualData): string => {
    if (!wasmLoaded) return ''
    
    try {
      const result = beemflowVisualToYaml(visual)
      return result.success ? result.data : ''
    } catch (error) {
      console.error('Failed to convert visual to YAML:', error)
      return ''
    }
  }, [wasmLoaded])

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
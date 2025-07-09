import { useCallback, useEffect, useState } from 'react'

// Simple WASM result interface
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

// Global WASM functions (set by Go runtime)
declare global {
  function beemflowParseYaml(yaml: string): WasmResult
  function beemflowValidateYaml(yaml: string): WasmResult
  function beemflowGenerateMermaid(yaml: string): WasmResult
  function beemflowYamlToVisual(yaml: string): WasmResult
  function beemflowVisualToYaml(visual: any): WasmResult
  class Go {
    importObject: WebAssembly.Imports
    run(instance: WebAssembly.Instance): Promise<void>
  }
}

export const useBeemFlow = () => {
  const [wasmLoaded, setWasmLoaded] = useState(false)
  const [wasmError, setWasmError] = useState<string | null>(null)

  // Load WASM once on mount
  useEffect(() => {
    let mounted = true
    
    const loadWasm = async () => {
      try {
        // Load wasm_exec.js
        const script = document.createElement('script')
        script.src = '/wasm_exec.js'
        document.head.appendChild(script)
        
        await new Promise((resolve, reject) => {
          script.onload = resolve
          script.onerror = reject
        })

        // Load and run WASM
        const go = new Go()
        const wasmModule = await WebAssembly.instantiateStreaming(
          fetch('/main.wasm'),
          go.importObject
        )
        
        go.run(wasmModule.instance)
        
        if (mounted) {
          setWasmLoaded(true)
        }
      } catch (error) {
        if (mounted) {
          setWasmError(`Failed to load WASM: ${error}`)
        }
      }
    }

    loadWasm()
    return () => { mounted = false }
  }, [])

  // Simple function wrappers
  const parseYaml = useCallback((yaml: string) => {
    if (!wasmLoaded) return null
    try {
      const result = beemflowParseYaml(yaml)
      return result.success ? result.data : null
    } catch (error) {
      console.error('Parse error:', error)
      return null
    }
  }, [wasmLoaded])

  const validateYaml = useCallback((yaml: string) => {
    if (!wasmLoaded) return { success: false, error: 'WASM not loaded' }
    try {
      return beemflowValidateYaml(yaml)
    } catch (error) {
      return { success: false, error: String(error) }
    }
  }, [wasmLoaded])

  const generateMermaid = useCallback((yaml: string) => {
    if (!wasmLoaded) return ''
    try {
      const result = beemflowGenerateMermaid(yaml)
      return result.success ? result.data : ''
    } catch (error) {
      console.error('Mermaid error:', error)
      return ''
    }
  }, [wasmLoaded])

  const yamlToVisual = useCallback((yaml: string): VisualData => {
    if (!wasmLoaded) return { nodes: [], edges: [], flow: null }
    try {
      const result = beemflowYamlToVisual(yaml)
      return result.success ? result.data : { nodes: [], edges: [], flow: null }
    } catch (error) {
      console.error('YAML to visual error:', error)
      return { nodes: [], edges: [], flow: null }
    }
  }, [wasmLoaded])

  const visualToYaml = useCallback((visual: VisualData) => {
    if (!wasmLoaded) return ''
    try {
      const result = beemflowVisualToYaml(visual)
      return result.success ? result.data : ''
    } catch (error) {
      console.error('Visual to YAML error:', error)
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
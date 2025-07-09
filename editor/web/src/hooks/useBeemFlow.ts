import { useCallback, useState } from 'react'



export interface VisualData {
  nodes: any[]
  edges: any[]
  flow: any
}

// HTTP API client for BeemFlow operations
class BeemFlowAPI {
  private baseUrl: string

  constructor(baseUrl: string = '') {
    this.baseUrl = baseUrl
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    })

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }

    return response.json()
  }

  async parseYaml(yaml: string) {
    return this.request('/editor/parse', {
      method: 'POST',
      body: JSON.stringify({ yaml }),
    })
  }

  async validateYaml(yaml: string) {
    return this.request('/validate', {
      method: 'POST',
      body: JSON.stringify({ name: yaml }),
    })
  }

  async generateMermaid(yaml: string) {
    return this.request('/flows/graph', {
      method: 'POST',
      body: JSON.stringify({ name: yaml }),
    })
  }

  async yamlToVisual(yaml: string) {
    return this.request('/editor/visual', {
      method: 'POST',
      body: JSON.stringify({ yaml }),
    })
  }

  async visualToYaml(visualData: VisualData) {
    return this.request('/editor/yaml', {
      method: 'POST',
      body: JSON.stringify({ visualData }),
    })
  }
}

export const useBeemFlow = () => {
  const [api] = useState(() => new BeemFlowAPI())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleRequest = useCallback(async <T>(
    requestFn: () => Promise<T>,
    defaultValue: T
  ): Promise<T> => {
    setLoading(true)
    setError(null)
    try {
      const result = await requestFn()
      return result
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err)
      setError(errorMessage)
      console.error('API error:', errorMessage)
      return defaultValue
    } finally {
      setLoading(false)
    }
  }, [])

  const parseYaml = useCallback(async (yaml: string) => {
    return handleRequest(() => api.parseYaml(yaml), null)
  }, [api, handleRequest])

  const validateYaml = useCallback(async (yaml: string) => {
    return handleRequest(
      () => api.validateYaml(yaml),
      { success: false, error: 'Validation failed' }
    )
  }, [api, handleRequest])

  const generateMermaid = useCallback(async (yaml: string) => {
    const result = await handleRequest(
      () => api.generateMermaid(yaml),
      { diagram: '' }
    )
    return (result as { diagram: string }).diagram || ''
  }, [api, handleRequest])

  const yamlToVisual = useCallback(async (yaml: string): Promise<VisualData> => {
    const result = await handleRequest(
      () => api.yamlToVisual(yaml),
      { nodes: [], edges: [], flow: null }
    )
    return result as VisualData
  }, [api, handleRequest])

  const visualToYaml = useCallback(async (visual: VisualData) => {
    const result = await handleRequest(() => api.visualToYaml(visual), '')
    return result as string
  }, [api, handleRequest])

  return {
    // API ready state (always true for HTTP API)
    wasmLoaded: true,
    wasmError: error,
    loading,
    error,
    
    // API methods (now async)
    parseYaml,
    validateYaml,
    generateMermaid,
    yamlToVisual,
    visualToYaml,
  }
}
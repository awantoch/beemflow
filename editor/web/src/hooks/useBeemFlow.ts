import { useCallback, useState } from 'react'

// Proper TypeScript interfaces matching backend types
export interface VisualData {
  nodes: VisualNode[]
  edges: VisualEdge[]
  flow?: Flow
}

export interface VisualNode {
  id: string
  type: string
  data: VisualNodeData
}

export interface VisualEdge {
  id: string
  source: string
  target: string
}

export interface VisualNodeData {
  id: string
  label: string
  use?: string
  with?: Record<string, any>
  if?: string
}

export interface Flow {
  name: string
  on: string
  steps: Step[]
}

export interface Step {
  id: string
  use: string
  with?: Record<string, any>
  if?: string
}

export interface ReactFlowData {
  nodes: ReactFlowNode[]
  edges: ReactFlowEdge[]
  flow: Flow
}

export interface ReactFlowNode {
  id: string
  type: string
  position: { x: number; y: number }
  data: VisualNodeData
}

export interface ReactFlowEdge {
  id: string
  source: string
  target: string
  label?: string
}

export interface ValidationResult {
  success: boolean
  error?: string
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

  async parseYaml(yaml: string): Promise<Flow> {
    return this.request<Flow>('/editor/parse', {
      method: 'POST',
      body: JSON.stringify({ yaml }),
    })
  }

  async validateYaml(yaml: string): Promise<ValidationResult> {
    return this.request<ValidationResult>('/validate', {
      method: 'POST',
      body: JSON.stringify({ name: yaml }),
    })
  }

  async generateMermaid(yaml: string): Promise<{ diagram: string }> {
    return this.request<{ diagram: string }>('/flows/graph', {
      method: 'POST',
      body: JSON.stringify({ name: yaml }),
    })
  }

  async yamlToVisual(yaml: string): Promise<ReactFlowData> {
    return this.request<ReactFlowData>('/editor/visual', {
      method: 'POST',
      body: JSON.stringify({ yaml }),
    })
  }

  async visualToYaml(visualData: VisualData): Promise<string> {
    return this.request<string>('/editor/yaml', {
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

  const parseYaml = useCallback(async (yaml: string): Promise<Flow | null> => {
    return handleRequest(() => api.parseYaml(yaml), null)
  }, [api, handleRequest])

  const validateYaml = useCallback(async (yaml: string): Promise<ValidationResult> => {
    return handleRequest(
      () => api.validateYaml(yaml),
      { success: false, error: 'Validation failed' }
    )
  }, [api, handleRequest])

  const generateMermaid = useCallback(async (yaml: string): Promise<string> => {
    const result = await handleRequest(
      () => api.generateMermaid(yaml),
      { diagram: '' }
    )
    return result.diagram || ''
  }, [api, handleRequest])

  const yamlToVisual = useCallback(async (yaml: string): Promise<ReactFlowData> => {
    return handleRequest(
      () => api.yamlToVisual(yaml),
      { nodes: [], edges: [], flow: { name: '', on: '', steps: [] } }
    )
  }, [api, handleRequest])

  const visualToYaml = useCallback(async (visual: VisualData): Promise<string> => {
    return handleRequest(() => api.visualToYaml(visual), '')
  }, [api, handleRequest])

  return {
    // API ready state (always true for HTTP API)
    wasmLoaded: true,
    wasmError: error,
    loading,
    error,
    
    // API methods (now async with proper types)
    parseYaml,
    validateYaml,
    generateMermaid,
    yamlToVisual,
    visualToYaml,
  }
}
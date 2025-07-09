import { useState, useCallback, useEffect } from 'react'
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  Connection,
} from 'reactflow'
import 'reactflow/dist/style.css'
import { Editor } from '@monaco-editor/react'
import { StepNode } from './components/StepNode'
import { useBeemFlow } from './hooks/useBeemFlow'

const nodeTypes = { stepNode: StepNode }

const initialYaml = `name: hello
on: cli.manual
steps:
  - id: greet
    use: core.echo
    with:
      text: "Hello, BeemFlow!"
  - id: again
    use: core.echo
    with:
      text: "{{ greet.text }} - from the editor!"`

function App() {
  const [yaml, setYaml] = useState(initialYaml)
  const [nodes, setNodes, onNodesChange] = useNodesState([])
  const [edges, setEdges, onEdgesChange] = useEdgesState([])
  const [editMode, setEditMode] = useState<'visual' | 'yaml' | 'split'>('split')
  const [validation, setValidation] = useState<{ success: boolean; error?: string } | null>(null)
  const [syncing, setSyncing] = useState(false)
  
  const { wasmError, yamlToVisual, visualToYaml, validateYaml, loading } = useBeemFlow()
  
  // Async YAML to visual sync
  const syncYamlToVisual = useCallback(async (yamlContent: string) => {
    if (!yamlContent.trim()) return
    
    setSyncing(true)
    try {
      const visual = await yamlToVisual(yamlContent)
      if (visual.nodes.length > 0) {
        setNodes(visual.nodes)
        setEdges(visual.edges)
      }
    } catch (error) {
      console.error('Failed to sync YAML to visual:', error)
    } finally {
      setSyncing(false)
    }
  }, [yamlToVisual, setNodes, setEdges])

  // Async visual to YAML sync
  const syncVisualToYaml = useCallback(async () => {
    if (nodes.length === 0) return
    
    setSyncing(true)
    try {
      const visual = { nodes, edges, flow: null }
      const newYaml = await visualToYaml(visual)
      if (newYaml && newYaml !== yaml) {
        setYaml(newYaml)
      }
    } catch (error) {
      console.error('Failed to sync visual to YAML:', error)
    } finally {
      setSyncing(false)
    }
  }, [nodes, edges, visualToYaml, yaml])

  // Async validation
  const validateYamlContent = useCallback(async (yamlContent: string) => {
    if (!yamlContent.trim()) {
      setValidation(null)
      return
    }
    
    try {
      const result = await validateYaml(yamlContent)
      setValidation(result as { success: boolean; error?: string })
    } catch (error) {
      setValidation({ success: false, error: String(error) })
    }
  }, [validateYaml])

  // Handle YAML changes
  const handleYamlChange = useCallback((value: string | undefined) => {
    const newYaml = value || ''
    setYaml(newYaml)
    
    // Debounce validation and sync
    const timeoutId = setTimeout(() => {
      validateYamlContent(newYaml)
      syncYamlToVisual(newYaml)
    }, 500)
    
    return () => clearTimeout(timeoutId)
  }, [validateYamlContent, syncYamlToVisual])

  // Handle visual changes with debouncing
  const handleNodesChange = useCallback((changes: any) => {
    onNodesChange(changes)
    const timeoutId = setTimeout(() => syncVisualToYaml(), 500)
    return () => clearTimeout(timeoutId)
  }, [onNodesChange, syncVisualToYaml])

  const handleEdgesChange = useCallback((changes: any) => {
    onEdgesChange(changes)
    const timeoutId = setTimeout(() => syncVisualToYaml(), 500)
    return () => clearTimeout(timeoutId)
  }, [onEdgesChange, syncVisualToYaml])

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds: any[]) => addEdge(params, eds)),
    [setEdges]
  )

  // Initial sync when component mounts
  useEffect(() => {
    validateYamlContent(yaml)
    syncYamlToVisual(yaml)
  }, []) // Only run once on mount

  // Loading states
  if (wasmError) {
    return (
      <div className="error-container">
        <div className="error-content">
          <h1>Failed to Load BeemFlow</h1>
          <p>{wasmError}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="app">
      {/* Header */}
      <header className="header">
        <div className="header-title">
          <h1>BeemFlow</h1>
          <span>→</span>
          <h2>Editor</h2>
        </div>
        
        <div className="header-controls">
          {/* Loading indicator */}
          {(loading || syncing) && (
            <div className="loading-indicator">
              <div className="spinner-small"></div>
              <span>Processing...</span>
            </div>
          )}
          
          {/* Validation indicator */}
          {validation && !loading && (
            <div className={`validation ${validation.success ? 'valid' : 'invalid'}`}>
              <div className="validation-dot"></div>
              {validation.success ? 'Valid' : 'Invalid'}
            </div>
          )}
          
          {/* View mode selector */}
          <div className="mode-selector">
            {(['visual', 'split', 'yaml'] as const).map((mode) => (
              <button
                key={mode}
                onClick={() => setEditMode(mode)}
                className={editMode === mode ? 'active' : ''}
              >
                {mode.charAt(0).toUpperCase() + mode.slice(1)}
              </button>
            ))}
          </div>
        </div>
      </header>

      {/* Main content */}
      <div className="main">
        {/* Visual Editor */}
        {(editMode === 'visual' || editMode === 'split') && (
          <div className="visual-editor">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={handleNodesChange}
              onEdgesChange={handleEdgesChange}
              onConnect={onConnect}
              nodeTypes={nodeTypes}
              fitView
            >
              <Background color="#f1f5f9" gap={20} size={1} />
              <Controls />
              <MiniMap nodeColor="#e2e8f0" />
            </ReactFlow>
          </div>
        )}

        {/* YAML Editor */}
        {(editMode === 'yaml' || editMode === 'split') && (
          <div className={`yaml-editor ${editMode === 'split' ? 'split' : ''}`}>
            <div className="yaml-header">
              <span>YAML Editor</span>
            </div>
            <div className="yaml-content">
              <Editor
                height="100%"
                language="yaml"
                theme="vs-light"
                value={yaml}
                onChange={handleYamlChange}
                options={{
                  minimap: { enabled: false },
                  fontSize: 14,
                  wordWrap: 'on',
                  automaticLayout: true,
                  scrollBeyondLastLine: false,
                  lineNumbers: 'on',
                  folding: true,
                }}
              />
            </div>
          </div>
        )}
      </div>

      <style>{`
        .app {
          height: 100vh;
          display: flex;
          flex-direction: column;
          background: #f3f4f6;
        }

        .header {
          height: 56px;
          background: white;
          border-bottom: 1px solid #e5e7eb;
          display: flex;
          align-items: center;
          padding: 0 24px;
          box-shadow: 0 1px 2px rgba(0,0,0,0.05);
        }

        .header-title {
          display: flex;
          align-items: center;
          gap: 12px;
        }

        .header-title h1 {
          font-size: 20px;
          font-weight: bold;
          color: #1f2937;
          margin: 0;
        }

        .header-title span {
          font-size: 18px;
          color: #9ca3af;
        }

        .header-title h2 {
          font-size: 18px;
          color: #4b5563;
          margin: 0;
        }

        .header-controls {
          margin-left: auto;
          display: flex;
          align-items: center;
          gap: 16px;
        }

        .loading-indicator {
          display: flex;
          align-items: center;
          gap: 8px;
          padding: 4px 12px;
          border-radius: 9999px;
          background: #f3f4f6;
          color: #6b7280;
          font-size: 14px;
        }

        .spinner-small {
          width: 16px;
          height: 16px;
          border: 2px solid #d1d5db;
          border-top: 2px solid #6b7280;
          border-radius: 50%;
          animation: spin 1s linear infinite;
        }

        .validation {
          display: flex;
          align-items: center;
          gap: 8px;
          padding: 4px 12px;
          border-radius: 9999px;
          font-size: 14px;
        }

        .validation.valid {
          background: #dcfce7;
          color: #166534;
        }

        .validation.invalid {
          background: #fee2e2;
          color: #991b1b;
        }

        .validation-dot {
          width: 8px;
          height: 8px;
          border-radius: 50%;
        }

        .validation.valid .validation-dot {
          background: #4ade80;
        }

        .validation.invalid .validation-dot {
          background: #f87171;
        }

        .mode-selector {
          display: flex;
          border-radius: 8px;
          border: 1px solid #d1d5db;
          overflow: hidden;
        }

        .mode-selector button {
          padding: 8px 16px;
          font-size: 14px;
          font-weight: 500;
          background: white;
          color: #374151;
          border: none;
          border-right: 1px solid #d1d5db;
          cursor: pointer;
        }

        .mode-selector button:last-child {
          border-right: none;
        }

        .mode-selector button.active {
          background: #2563eb;
          color: white;
        }

        .main {
          flex: 1;
          display: flex;
          overflow: hidden;
        }

        .visual-editor {
          flex: 1;
          background: white;
        }

        .yaml-editor {
          flex: 1;
          background: white;
          display: flex;
          flex-direction: column;
        }

        .yaml-editor.split {
          border-left: 1px solid #e5e7eb;
        }

        .yaml-header {
          height: 40px;
          background: #f9fafb;
          border-bottom: 1px solid #e5e7eb;
          display: flex;
          align-items: center;
          padding: 0 16px;
          font-size: 14px;
          font-weight: 500;
          color: #374151;
        }

        .yaml-content {
          flex: 1;
        }

        .loading-container, .error-container {
          height: 100vh;
          display: flex;
          align-items: center;
          justify-content: center;
        }

        .loading-container {
          background: #eff6ff;
        }

        .error-container {
          background: #fef2f2;
        }

        .loading-content, .error-content {
          text-align: center;
          padding: 32px;
        }

        .loading-content h1 {
          font-size: 24px;
          font-weight: bold;
          color: #1e40af;
          margin: 16px 0 8px 0;
        }

        .error-content h1 {
          font-size: 24px;
          font-weight: bold;
          color: #991b1b;
          margin: 0 0 16px 0;
        }

        .loading-content p {
          color: #2563eb;
          margin: 0;
        }

        .error-content p {
          color: #dc2626;
          margin: 0;
        }

        .spinner {
          width: 48px;
          height: 48px;
          border: 3px solid #2563eb;
          border-top: 3px solid transparent;
          border-radius: 50%;
          animation: spin 1s linear infinite;
          margin: 0 auto;
        }

        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  )
}

export default App
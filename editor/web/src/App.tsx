import { useState, useCallback, useMemo } from 'react'
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

const nodeTypes = {
  stepNode: StepNode,
}

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
  
  const { wasmLoaded, wasmError, yamlToVisual, visualToYaml, validateYaml } = useBeemFlow()
  
  // Sync YAML to visual
  const handleYamlChange = useCallback((value: string | undefined) => {
    const newYaml = value || ''
    setYaml(newYaml)
    
    if (wasmLoaded && newYaml.trim()) {
      const visual = yamlToVisual(newYaml)
      if (visual.nodes.length > 0) {
        setNodes(visual.nodes)
        setEdges(visual.edges)
      }
    }
  }, [wasmLoaded, yamlToVisual, setNodes, setEdges])
  
  // Sync visual to YAML
  const handleVisualChange = useCallback(() => {
    if (wasmLoaded && nodes.length > 0) {
      const visual = { nodes, edges, flow: null }
      const newYaml = visualToYaml(visual)
      if (newYaml && newYaml !== yaml) {
        setYaml(newYaml)
      }
    }
  }, [wasmLoaded, nodes, edges, visualToYaml, yaml])
  
  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  )
  
  // Validate YAML
  const validation = useMemo(() => {
    if (!wasmLoaded || !yaml.trim()) return null
    return validateYaml(yaml)
  }, [wasmLoaded, yaml, validateYaml])
  
  // Debounced visual change handlers
  const handleNodesChange = useCallback((changes: any) => {
    onNodesChange(changes)
    setTimeout(handleVisualChange, 100)
  }, [onNodesChange, handleVisualChange])
  
  const handleEdgesChange = useCallback((changes: any) => {
    onEdgesChange(changes)
    setTimeout(handleVisualChange, 100)
  }, [onEdgesChange, handleVisualChange])

  // Initial sync when WASM loads
  useMemo(() => {
    if (wasmLoaded && yaml.trim()) {
      const visual = yamlToVisual(yaml)
      if (visual.nodes.length > 0) {
        setNodes(visual.nodes)
        setEdges(visual.edges)
      }
    }
  }, [wasmLoaded]) // Only run when WASM loads

  if (wasmError) {
    return (
      <div style={{
        height: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: '#fef2f2'
      }}>
        <div style={{ textAlign: 'center', padding: '32px' }}>
          <h1 style={{ fontSize: '24px', fontWeight: 'bold', color: '#991b1b', marginBottom: '16px' }}>
            Failed to Load BeemFlow
          </h1>
          <p style={{ color: '#dc2626' }}>{wasmError}</p>
        </div>
      </div>
    )
  }

  if (!wasmLoaded) {
    return (
      <div style={{
        height: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: '#eff6ff'
      }}>
        <div style={{ textAlign: 'center', padding: '32px' }}>
          <div style={{
            width: '48px',
            height: '48px',
            border: '3px solid #2563eb',
            borderTop: '3px solid transparent',
            borderRadius: '50%',
            animation: 'spin 1s linear infinite',
            margin: '0 auto 16px'
          }}></div>
          <h1 style={{ fontSize: '24px', fontWeight: 'bold', color: '#1e40af', marginBottom: '8px' }}>
            Loading BeemFlow
          </h1>
          <p style={{ color: '#2563eb' }}>Initializing WASM runtime...</p>
        </div>
      </div>
    )
  }

  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', backgroundColor: '#f3f4f6' }}>
      {/* Header */}
      <header style={{
        height: '56px',
        backgroundColor: 'white',
        borderBottom: '1px solid #e5e7eb',
        display: 'flex',
        alignItems: 'center',
        padding: '0 24px',
        boxShadow: '0 1px 2px 0 rgba(0, 0, 0, 0.05)'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <h1 style={{ fontSize: '20px', fontWeight: 'bold', color: '#1f2937' }}>BeemFlow</h1>
          <span style={{ fontSize: '18px', color: '#9ca3af' }}>→</span>
          <h2 style={{ fontSize: '18px', color: '#4b5563' }}>Editor</h2>
        </div>
        
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '16px' }}>
          {/* Validation indicator */}
          {validation && (
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              padding: '4px 12px',
              borderRadius: '9999px',
              fontSize: '14px',
              backgroundColor: validation.success ? '#dcfce7' : '#fee2e2',
              color: validation.success ? '#166534' : '#991b1b'
            }}>
              <div style={{
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                backgroundColor: validation.success ? '#4ade80' : '#f87171'
              }}></div>
              {validation.success ? 'Valid' : 'Invalid'}
            </div>
          )}
          
          {/* View mode selector */}
          <div style={{ display: 'flex', borderRadius: '8px', border: '1px solid #d1d5db', overflow: 'hidden' }}>
            {(['visual', 'split', 'yaml'] as const).map((mode) => (
              <button
                key={mode}
                onClick={() => setEditMode(mode)}
                style={{
                  padding: '8px 16px',
                  fontSize: '14px',
                  fontWeight: 500,
                  backgroundColor: editMode === mode ? '#2563eb' : 'white',
                  color: editMode === mode ? 'white' : '#374151',
                  border: 'none',
                  borderRight: mode !== 'yaml' ? '1px solid #d1d5db' : 'none',
                  cursor: 'pointer'
                }}
              >
                {mode.charAt(0).toUpperCase() + mode.slice(1)}
              </button>
            ))}
          </div>
        </div>
      </header>

      {/* Main content */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* Visual Editor */}
        {(editMode === 'visual' || editMode === 'split') && (
          <div style={{ flex: 1, backgroundColor: '#f9fafb' }}>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={handleNodesChange}
              onEdgesChange={handleEdgesChange}
              onConnect={onConnect}
              nodeTypes={nodeTypes}
              fitView
              style={{ backgroundColor: 'white' }}
            >
              <Background color="#f1f5f9" gap={20} size={1} />
              <Controls style={{ backgroundColor: 'white', border: '1px solid #d1d5db', borderRadius: '8px', boxShadow: '0 10px 15px -3px rgba(0, 0, 0, 0.1)' }} />
              <MiniMap 
                nodeColor="#e2e8f0"
                style={{ backgroundColor: 'white', border: '1px solid #d1d5db', borderRadius: '8px', boxShadow: '0 10px 15px -3px rgba(0, 0, 0, 0.1)' }}
              />
            </ReactFlow>
          </div>
        )}

        {/* YAML Editor */}
        {(editMode === 'yaml' || editMode === 'split') && (
          <div style={{ flex: 1, backgroundColor: 'white', borderLeft: editMode === 'split' ? '1px solid #e5e7eb' : 'none' }}>
            <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
              <div style={{
                height: '40px',
                backgroundColor: '#f9fafb',
                borderBottom: '1px solid #e5e7eb',
                display: 'flex',
                alignItems: 'center',
                padding: '0 16px'
              }}>
                <span style={{ fontSize: '14px', fontWeight: 500, color: '#374151' }}>YAML Editor</span>
              </div>
              <div style={{ flex: 1 }}>
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
                    renderLineHighlight: 'none',
                    overviewRulerBorder: false,
                    hideCursorInOverviewRuler: true,
                    lineNumbers: 'on',
                    glyphMargin: false,
                    folding: true,
                    lineDecorationsWidth: 0,
                    lineNumbersMinChars: 3,
                  }}
                />
              </div>
            </div>
          </div>
        )}
      </div>

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  )
}

export default App
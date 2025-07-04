import { useState, useCallback, useMemo } from 'react'
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  Connection,
  Node,
  Edge,
} from '@reactflow/core'
import '@reactflow/core/dist/style.css'
import { Editor } from '@monaco-editor/react'
import { StepNode } from './components/StepNode'
import { useBeemFlow } from './hooks/useBeemFlow'
import './App.css'

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
      text: "{{ greet.text }} - from visual editor!"`

function App() {
  const [yaml, setYaml] = useState(initialYaml)
  const [nodes, setNodes, onNodesChange] = useNodesState([])
  const [edges, setEdges, onEdgesChange] = useEdgesState([])
  const [editMode, setEditMode] = useState<'visual' | 'yaml' | 'split'>('split')
  
  const { wasmLoaded, wasmError, yamlToVisual, visualToYaml, validateYaml } = useBeemFlow()
  
  // Sync YAML to visual when WASM loads or YAML changes
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
  
  // Initial sync when WASM loads
  const handleWasmLoad = useCallback(() => {
    if (yaml.trim()) {
      handleYamlChange(yaml)
    }
  }, [yaml, handleYamlChange])
  
  // Sync visual to YAML when nodes/edges change
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
  
  // Sync visual changes
  const handleNodesChange = useCallback((changes: any) => {
    onNodesChange(changes)
    setTimeout(handleVisualChange, 100) // Debounce
  }, [onNodesChange, handleVisualChange])
  
  const handleEdgesChange = useCallback((changes: any) => {
    onEdgesChange(changes)
    setTimeout(handleVisualChange, 100) // Debounce
  }, [onEdgesChange, handleVisualChange])

  if (wasmError) {
    return (
      <div className="h-screen flex items-center justify-center bg-red-50">
        <div className="text-center p-8">
          <h1 className="text-2xl font-bold text-red-800 mb-4">Failed to Load BeemFlow</h1>
          <p className="text-red-600">{wasmError}</p>
        </div>
      </div>
    )
  }

  if (!wasmLoaded) {
    return (
      <div className="h-screen flex items-center justify-center bg-blue-50">
        <div className="text-center p-8">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
          <h1 className="text-2xl font-bold text-blue-800 mb-2">Loading BeemFlow</h1>
          <p className="text-blue-600">Initializing WASM runtime...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="h-screen flex flex-col bg-gray-100">
      {/* Header */}
      <header className="h-14 bg-white border-b border-gray-200 flex items-center px-6 shadow-sm">
        <div className="flex items-center gap-3">
          <h1 className="text-xl font-bold text-gray-800">BeemFlow</h1>
          <span className="text-lg text-gray-400">→</span>
          <h2 className="text-lg text-gray-600">Visual Editor</h2>
        </div>
        
        <div className="ml-auto flex items-center gap-4">
          {/* Validation indicator */}
          {validation && (
            <div className={`flex items-center gap-2 px-3 py-1 rounded-full text-sm ${
              validation.valid 
                ? 'bg-green-100 text-green-800' 
                : 'bg-red-100 text-red-800'
            }`}>
              <div className={`w-2 h-2 rounded-full ${
                validation.valid ? 'bg-green-400' : 'bg-red-400'
              }`}></div>
              {validation.valid ? 'Valid' : 'Invalid'}
            </div>
          )}
          
          {/* View mode selector */}
          <div className="flex rounded-lg border border-gray-300 overflow-hidden">
            <button
              onClick={() => setEditMode('visual')}
              className={`px-4 py-2 text-sm font-medium ${
                editMode === 'visual'
                  ? 'bg-blue-600 text-white'
                  : 'bg-white text-gray-700 hover:bg-gray-50'
              }`}
            >
              Visual
            </button>
            <button
              onClick={() => setEditMode('split')}
              className={`px-4 py-2 text-sm font-medium border-l border-gray-300 ${
                editMode === 'split'
                  ? 'bg-blue-600 text-white'
                  : 'bg-white text-gray-700 hover:bg-gray-50'
              }`}
            >
              Split
            </button>
            <button
              onClick={() => setEditMode('yaml')}
              className={`px-4 py-2 text-sm font-medium border-l border-gray-300 ${
                editMode === 'yaml'
                  ? 'bg-blue-600 text-white'
                  : 'bg-white text-gray-700 hover:bg-gray-50'
              }`}
            >
              YAML
            </button>
          </div>
        </div>
      </header>

      {/* Main content */}
      <div className="flex-1 flex overflow-hidden">
        {/* Visual Editor */}
        {(editMode === 'visual' || editMode === 'split') && (
          <div className="flex-1 bg-gray-50">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={handleNodesChange}
              onEdgesChange={handleEdgesChange}
              onConnect={onConnect}
              nodeTypes={nodeTypes}
              fitView
              className="bg-white"
            >
              <Background color="#f1f5f9" gap={20} size={1} />
              <Controls className="bg-white border border-gray-300 rounded-lg shadow-lg" />
              <MiniMap 
                nodeColor="#e2e8f0"
                className="bg-white border border-gray-300 rounded-lg shadow-lg"
              />
            </ReactFlow>
          </div>
        )}

        {/* YAML Editor */}
        {(editMode === 'yaml' || editMode === 'split') && (
          <div className="flex-1 bg-white border-l border-gray-200">
            <div className="h-full flex flex-col">
              <div className="h-10 bg-gray-50 border-b border-gray-200 flex items-center px-4">
                <span className="text-sm font-medium text-gray-700">YAML Editor</span>
              </div>
              <div className="flex-1">
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
    </div>
  )
}

export default App
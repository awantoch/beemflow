# BeemFlow Visual Editor: Implementation Guide

> **Quick Start**: A practical guide for developers to build the BeemFlow Visual Editor from the ground up.

---

## Phase 1: Proof of Concept (3-5 days)

### 1. Backend API Extensions

First, extend BeemFlow's HTTP server to support the visual editor:

```go
// Add to http/server.go
func (s *Server) setupVisualEditorRoutes() {
    // Flow parsing and generation
    s.router.POST("/api/v1/visual/parse", s.handleParseFlow)
    s.router.POST("/api/v1/visual/generate", s.handleGenerateFlow)
    s.router.POST("/api/v1/visual/validate", s.handleValidateFlow)
    
    // Tool registry
    s.router.GET("/api/v1/visual/tools", s.handleGetTools)
    s.router.GET("/api/v1/visual/tools/:id", s.handleGetTool)
}

// Visual editor specific types
type VisualNode struct {
    ID       string            `json:"id"`
    Type     string            `json:"type"`
    Position struct {
        X float64 `json:"x"`
        Y float64 `json:"y"`
    } `json:"position"`
    Data     map[string]any    `json:"data"`
}

type VisualEdge struct {
    ID     string `json:"id"`
    Source string `json:"source"`
    Target string `json:"target"`
    Label  string `json:"label,omitempty"`
}

type VisualFlow struct {
    Nodes []VisualNode `json:"nodes"`
    Edges []VisualEdge `json:"edges"`
    Flow  model.Flow   `json:"flow"`
}

// Parse YAML to visual format
func (s *Server) handleParseFlow(w http.ResponseWriter, r *http.Request) {
    var req struct {
        YAML string `json:"yaml"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Parse YAML to Flow
    flow, err := dsl.ParseFromString(req.YAML)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Convert to visual format
    visualFlow := convertFlowToVisual(flow)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(visualFlow)
}

// Generate YAML from visual format
func (s *Server) handleGenerateFlow(w http.ResponseWriter, r *http.Request) {
    var visualFlow VisualFlow
    
    if err := json.NewDecoder(r.Body).Decode(&visualFlow); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Convert visual to Flow
    flow := convertVisualToFlow(visualFlow)
    
    // Generate YAML
    yamlData, err := yaml.Marshal(flow)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    response := struct {
        YAML string `json:"yaml"`
    }{
        YAML: string(yamlData),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### 2. Visual ↔ Flow Conversion

Create conversion utilities:

```go
// Convert Flow to visual representation
func convertFlowToVisual(flow *model.Flow) VisualFlow {
    var nodes []VisualNode
    var edges []VisualEdge
    
    // Convert each step to a node
    for i, step := range flow.Steps {
        node := VisualNode{
            ID:   step.ID,
            Type: step.Use,
            Position: struct {
                X float64 `json:"x"`
                Y float64 `json:"y"`
            }{
                X: float64(i * 300), // Basic positioning
                Y: 100,
            },
            Data: map[string]any{
                "id":   step.ID,
                "use":  step.Use,
                "with": step.With,
                "if":   step.If,
            },
        }
        nodes = append(nodes, node)
        
        // Create edges based on dependencies
        if len(step.DependsOn) > 0 {
            for _, dep := range step.DependsOn {
                edge := VisualEdge{
                    ID:     fmt.Sprintf("%s-%s", dep, step.ID),
                    Source: dep,
                    Target: step.ID,
                }
                edges = append(edges, edge)
            }
        } else if i > 0 {
            // Sequential dependency
            prevStep := flow.Steps[i-1]
            edge := VisualEdge{
                ID:     fmt.Sprintf("%s-%s", prevStep.ID, step.ID),
                Source: prevStep.ID,
                Target: step.ID,
            }
            edges = append(edges, edge)
        }
    }
    
    return VisualFlow{
        Nodes: nodes,
        Edges: edges,
        Flow:  *flow,
    }
}

// Convert visual representation to Flow
func convertVisualToFlow(visual VisualFlow) *model.Flow {
    var steps []model.Step
    
    // Sort nodes by position (simple left-to-right ordering)
    sort.Slice(visual.Nodes, func(i, j int) bool {
        return visual.Nodes[i].Position.X < visual.Nodes[j].Position.X
    })
    
    // Create dependency map from edges
    depMap := make(map[string][]string)
    for _, edge := range visual.Edges {
        depMap[edge.Target] = append(depMap[edge.Target], edge.Source)
    }
    
    // Convert nodes to steps
    for _, node := range visual.Nodes {
        step := model.Step{
            ID:        node.ID,
            Use:       node.Type,
            DependsOn: depMap[node.ID],
        }
        
        // Extract configuration from node data
        if with, ok := node.Data["with"].(map[string]any); ok {
            step.With = with
        }
        if ifCond, ok := node.Data["if"].(string); ok {
            step.If = ifCond
        }
        
        steps = append(steps, step)
    }
    
    return &model.Flow{
        Name:  visual.Flow.Name,
        On:    visual.Flow.On,
        Vars:  visual.Flow.Vars,
        Steps: steps,
    }
}
```

### 3. Frontend Setup

Create the React frontend:

```bash
# Create new React project
npm create vite@latest beemflow-visual-editor -- --template react-ts
cd beemflow-visual-editor

# Install dependencies
npm install @reactflow/core @reactflow/node-resizer @reactflow/minimap @reactflow/controls
npm install @monaco-editor/react
npm install @tanstack/react-query
npm install zustand
npm install tailwindcss
npm install lucide-react
```

### 4. Core Components

Create the main visual editor:

```typescript
// App.tsx
import { useCallback, useMemo, useState } from 'react'
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  Node,
  Edge,
  Connection,
} from '@reactflow/core'
import '@reactflow/core/dist/style.css'
import { MonacoYamlEditor } from './components/MonacoYamlEditor'
import { ToolLibrary } from './components/ToolLibrary'
import { StepNode } from './components/StepNode'
import { useSyncEngine } from './hooks/useSyncEngine'
import './App.css'

const nodeTypes = {
  stepNode: StepNode,
}

const initialNodes: Node[] = [
  {
    id: '1',
    type: 'stepNode',
    position: { x: 100, y: 100 },
    data: { 
      id: 'start', 
      use: 'core.echo', 
      with: { text: 'Hello World' } 
    },
  },
]

const initialEdges: Edge[] = []

function App() {
  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)
  const [yaml, setYaml] = useState('')
  const [editMode, setEditMode] = useState<'visual' | 'yaml' | 'split'>('split')
  
  const { syncYamlToVisual, syncVisualToYaml } = useSyncEngine()
  
  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  )
  
  const handleYamlChange = useCallback((value: string) => {
    setYaml(value)
    // Sync to visual
    const { nodes: newNodes, edges: newEdges } = syncYamlToVisual(value)
    setNodes(newNodes)
    setEdges(newEdges)
  }, [setNodes, setEdges, syncYamlToVisual])
  
  const handleVisualChange = useCallback(() => {
    // Sync to YAML
    const newYaml = syncVisualToYaml(nodes, edges)
    setYaml(newYaml)
  }, [nodes, edges, syncVisualToYaml])
  
  return (
    <div className="h-screen flex flex-col">
      {/* Header */}
      <div className="h-12 bg-gray-800 text-white flex items-center px-4">
        <h1 className="text-lg font-semibold">BeemFlow Visual Editor</h1>
        <div className="ml-auto flex gap-2">
          <button 
            onClick={() => setEditMode('visual')}
            className={`px-3 py-1 rounded ${editMode === 'visual' ? 'bg-blue-600' : 'bg-gray-600'}`}
          >
            Visual
          </button>
          <button 
            onClick={() => setEditMode('split')}
            className={`px-3 py-1 rounded ${editMode === 'split' ? 'bg-blue-600' : 'bg-gray-600'}`}
          >
            Split
          </button>
          <button 
            onClick={() => setEditMode('yaml')}
            className={`px-3 py-1 rounded ${editMode === 'yaml' ? 'bg-blue-600' : 'bg-gray-600'}`}
          >
            YAML
          </button>
        </div>
      </div>
      
      {/* Main Content */}
      <div className="flex-1 flex">
        <ToolLibrary />
        
        {/* Visual Editor */}
        {(editMode === 'visual' || editMode === 'split') && (
          <div className="flex-1 bg-gray-50">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              nodeTypes={nodeTypes}
              fitView
            >
              <Background />
              <Controls />
              <MiniMap />
            </ReactFlow>
          </div>
        )}
        
        {/* YAML Editor */}
        {(editMode === 'yaml' || editMode === 'split') && (
          <div className="flex-1">
            <MonacoYamlEditor 
              value={yaml}
              onChange={handleYamlChange}
            />
          </div>
        )}
      </div>
    </div>
  )
}

export default App
```

### 5. Step Node Component

Create custom nodes for different step types:

```typescript
// components/StepNode.tsx
import { Handle, Position, NodeProps } from '@reactflow/core'
import { memo } from 'react'

interface StepNodeData {
  id: string
  use: string
  with?: Record<string, any>
  if?: string
}

export const StepNode = memo(({ data }: NodeProps<StepNodeData>) => {
  const getNodeIcon = (use: string) => {
    if (use.includes('echo')) return '📢'
    if (use.includes('http')) return '🌐'
    if (use.includes('openai')) return '🤖'
    if (use.includes('slack')) return '💬'
    return '⚙️'
  }
  
  const getNodeColor = (use: string) => {
    if (use.includes('echo')) return 'bg-blue-100 border-blue-300'
    if (use.includes('http')) return 'bg-red-100 border-red-300'
    if (use.includes('openai')) return 'bg-green-100 border-green-300'
    if (use.includes('slack')) return 'bg-purple-100 border-purple-300'
    return 'bg-gray-100 border-gray-300'
  }
  
  return (
    <div className={`px-4 py-2 shadow-md rounded-md border-2 ${getNodeColor(data.use)}`}>
      <Handle 
        type="target" 
        position={Position.Top} 
        className="w-3 h-3"
      />
      
      <div className="flex items-center gap-2">
        <span className="text-lg">{getNodeIcon(data.use)}</span>
        <div>
          <div className="font-semibold text-sm">{data.id}</div>
          <div className="text-xs text-gray-600">{data.use}</div>
        </div>
      </div>
      
      {data.with && (
        <div className="mt-1 text-xs text-gray-500">
          {Object.keys(data.with).length} parameters
        </div>
      )}
      
      {data.if && (
        <div className="mt-1 text-xs text-yellow-600">
          Conditional
        </div>
      )}
      
      <Handle 
        type="source" 
        position={Position.Bottom} 
        className="w-3 h-3"
      />
    </div>
  )
})
```

### 6. YAML Editor Component

Integrate Monaco Editor with YAML support:

```typescript
// components/MonacoYamlEditor.tsx
import { Editor } from '@monaco-editor/react'
import { useEffect, useRef } from 'react'

interface MonacoYamlEditorProps {
  value: string
  onChange: (value: string) => void
}

export const MonacoYamlEditor = ({ value, onChange }: MonacoYamlEditorProps) => {
  const editorRef = useRef<any>(null)
  
  const handleEditorDidMount = (editor: any, monaco: any) => {
    editorRef.current = editor
    
    // Configure YAML language
    monaco.languages.yaml?.yamlDefaults.setDiagnosticsOptions({
      validate: true,
      schemas: [
        {
          uri: 'beemflow://schema.yaml',
          fileMatch: ['*'],
          schema: {
            type: 'object',
            properties: {
              name: { type: 'string' },
              on: { type: 'string' },
              vars: { type: 'object' },
              steps: {
                type: 'array',
                items: {
                  type: 'object',
                  properties: {
                    id: { type: 'string' },
                    use: { type: 'string' },
                    with: { type: 'object' },
                  },
                  required: ['id', 'use'],
                },
              },
            },
            required: ['name', 'steps'],
          },
        },
      ],
    })
  }
  
  return (
    <div className="h-full">
      <div className="h-8 bg-gray-100 border-b flex items-center px-4">
        <span className="text-sm font-medium">YAML Editor</span>
      </div>
      <div className="h-full">
        <Editor
          height="100%"
          language="yaml"
          theme="vs-dark"
          value={value}
          onChange={(value) => onChange(value || '')}
          onMount={handleEditorDidMount}
          options={{
            minimap: { enabled: false },
            fontSize: 14,
            wordWrap: 'on',
            automaticLayout: true,
          }}
        />
      </div>
    </div>
  )
}
```

### 7. Tool Library Component

Create a searchable tool library:

```typescript
// components/ToolLibrary.tsx
import { useState, useMemo } from 'react'
import { Search, Package, Bot, Globe, MessageSquare } from 'lucide-react'

interface Tool {
  id: string
  name: string
  description: string
  category: string
  icon: React.ReactNode
}

const tools: Tool[] = [
  {
    id: 'core.echo',
    name: 'Echo',
    description: 'Print text to output',
    category: 'core',
    icon: <Package className="w-4 h-4" />,
  },
  {
    id: 'http.fetch',
    name: 'HTTP Fetch',
    description: 'Fetch data from URL',
    category: 'http',
    icon: <Globe className="w-4 h-4" />,
  },
  {
    id: 'openai.chat_completion',
    name: 'OpenAI Chat',
    description: 'Chat with OpenAI models',
    category: 'ai',
    icon: <Bot className="w-4 h-4" />,
  },
  {
    id: 'slack.chat.postMessage',
    name: 'Slack Message',
    description: 'Send message to Slack',
    category: 'integrations',
    icon: <MessageSquare className="w-4 h-4" />,
  },
]

export const ToolLibrary = () => {
  const [searchTerm, setSearchTerm] = useState('')
  
  const filteredTools = useMemo(() => {
    return tools.filter(tool => 
      tool.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      tool.description.toLowerCase().includes(searchTerm.toLowerCase())
    )
  }, [searchTerm])
  
  const handleDragStart = (event: React.DragEvent, tool: Tool) => {
    event.dataTransfer.setData('application/reactflow', JSON.stringify(tool))
  }
  
  return (
    <div className="w-64 bg-white border-r border-gray-200 flex flex-col">
      <div className="p-4 border-b border-gray-200">
        <div className="relative">
          <Search className="w-4 h-4 absolute left-3 top-3 text-gray-400" />
          <input
            type="text"
            placeholder="Search tools..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>
      
      <div className="flex-1 overflow-y-auto">
        <div className="p-4 space-y-2">
          {filteredTools.map(tool => (
            <div
              key={tool.id}
              draggable
              onDragStart={(e) => handleDragStart(e, tool)}
              className="p-3 border border-gray-200 rounded-lg cursor-move hover:bg-gray-50 transition-colors"
            >
              <div className="flex items-center gap-2 mb-1">
                {tool.icon}
                <span className="font-medium text-sm">{tool.name}</span>
              </div>
              <p className="text-xs text-gray-600">{tool.description}</p>
              <div className="mt-1">
                <span className="inline-block px-2 py-1 bg-gray-100 text-xs rounded">
                  {tool.category}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
```

### 8. Sync Engine Hook

Create the bidirectional sync logic:

```typescript
// hooks/useSyncEngine.ts
import { useCallback } from 'react'
import { Node, Edge } from '@reactflow/core'

interface FlowData {
  name: string
  on: string
  vars?: Record<string, any>
  steps: Array<{
    id: string
    use: string
    with?: Record<string, any>
    if?: string
  }>
}

export const useSyncEngine = () => {
  const syncYamlToVisual = useCallback((yamlString: string) => {
    try {
      // Parse YAML (you'd use a proper YAML parser in production)
      const flow: FlowData = JSON.parse(yamlString) // Simplified
      
      const nodes: Node[] = flow.steps.map((step, index) => ({
        id: step.id,
        type: 'stepNode',
        position: { x: 100 + index * 300, y: 100 },
        data: {
          id: step.id,
          use: step.use,
          with: step.with,
          if: step.if,
        },
      }))
      
      const edges: Edge[] = []
      // Create sequential edges
      for (let i = 1; i < flow.steps.length; i++) {
        edges.push({
          id: `${flow.steps[i-1].id}-${flow.steps[i].id}`,
          source: flow.steps[i-1].id,
          target: flow.steps[i].id,
        })
      }
      
      return { nodes, edges }
    } catch (error) {
      console.error('Error parsing YAML:', error)
      return { nodes: [], edges: [] }
    }
  }, [])
  
  const syncVisualToYaml = useCallback((nodes: Node[], edges: Edge[]) => {
    try {
      const steps = nodes.map(node => ({
        id: node.data.id,
        use: node.data.use,
        with: node.data.with,
        if: node.data.if,
      }))
      
      const flow: FlowData = {
        name: 'visual_flow',
        on: 'cli.manual',
        steps,
      }
      
      // Generate YAML (simplified - use proper YAML library)
      return JSON.stringify(flow, null, 2)
    } catch (error) {
      console.error('Error generating YAML:', error)
      return ''
    }
  }, [])
  
  return { syncYamlToVisual, syncVisualToYaml }
}
```

---

## Phase 2: Backend Integration

### 1. Add Visual Editor Routes to BeemFlow

Add these routes to your existing BeemFlow HTTP server:

```go
// In http/server.go, add to setupRoutes()
func (s *Server) setupRoutes() {
    // ... existing routes ...
    
    // Visual editor routes
    s.setupVisualEditorRoutes()
}

func (s *Server) setupVisualEditorRoutes() {
    v1 := s.router.PathPrefix("/api/v1/visual").Subrouter()
    
    v1.HandleFunc("/parse", s.handleParseFlow).Methods("POST")
    v1.HandleFunc("/generate", s.handleGenerateFlow).Methods("POST")
    v1.HandleFunc("/validate", s.handleValidateFlow).Methods("POST")
    v1.HandleFunc("/tools", s.handleGetTools).Methods("GET")
    v1.HandleFunc("/tools/{id}", s.handleGetTool).Methods("GET")
}
```

### 2. Test the Integration

Create a simple test to verify the integration:

```bash
# Test parsing a flow
curl -X POST http://localhost:8080/api/v1/visual/parse \
  -H "Content-Type: application/json" \
  -d '{"yaml": "name: test\non: cli.manual\nsteps:\n  - id: echo\n    use: core.echo\n    with:\n      text: hello"}'

# Test generating YAML
curl -X POST http://localhost:8080/api/v1/visual/generate \
  -H "Content-Type: application/json" \
  -d '{"nodes": [{"id": "echo", "type": "core.echo", "position": {"x": 100, "y": 100}, "data": {"id": "echo", "use": "core.echo", "with": {"text": "hello"}}}], "edges": [], "flow": {"name": "test", "on": "cli.manual"}}'
```

---

## Phase 3: Next Steps

### 1. Enhanced Node Types

Add more sophisticated node types:

```typescript
// components/nodes/ParallelNode.tsx
export const ParallelNode = ({ data }: NodeProps) => {
  return (
    <div className="bg-pink-100 border-pink-300 border-2 rounded-lg p-4">
      <div className="flex items-center gap-2 mb-2">
        <span className="text-lg">⚡</span>
        <span className="font-semibold">Parallel</span>
      </div>
      <div className="text-xs text-gray-600">
        {data.steps?.length || 0} steps
      </div>
    </div>
  )
}

// components/nodes/ConditionalNode.tsx
export const ConditionalNode = ({ data }: NodeProps) => {
  return (
    <div className="bg-yellow-100 border-yellow-300 border-2 rounded-lg p-4">
      <div className="flex items-center gap-2 mb-2">
        <span className="text-lg">❓</span>
        <span className="font-semibold">If</span>
      </div>
      <div className="text-xs text-gray-600">
        {data.if}
      </div>
    </div>
  )
}
```

### 2. Property Panels

Add configuration panels for selected nodes:

```typescript
// components/PropertyPanel.tsx
export const PropertyPanel = ({ selectedNode, onUpdate }: PropertyPanelProps) => {
  if (!selectedNode) return null
  
  return (
    <div className="w-80 bg-white border-l border-gray-200 p-4">
      <h3 className="font-semibold mb-4">Properties</h3>
      
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-1">ID</label>
          <input 
            type="text"
            value={selectedNode.data.id}
            onChange={(e) => onUpdate('id', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2"
          />
        </div>
        
        <div>
          <label className="block text-sm font-medium mb-1">Use</label>
          <input 
            type="text"
            value={selectedNode.data.use}
            onChange={(e) => onUpdate('use', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2"
          />
        </div>
        
        <div>
          <label className="block text-sm font-medium mb-1">Parameters</label>
          <JsonEditor 
            value={selectedNode.data.with || {}}
            onChange={(value) => onUpdate('with', value)}
          />
        </div>
      </div>
    </div>
  )
}
```

### 3. Real-time Collaboration

Add WebSocket support for collaborative editing:

```go
// Add WebSocket handler
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true // Configure properly in production
        },
    }
    
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("WebSocket upgrade error: %v", err)
        return
    }
    defer conn.Close()
    
    // Handle collaborative editing messages
    for {
        var msg CollaborationMessage
        if err := conn.ReadJSON(&msg); err != nil {
            log.Printf("WebSocket read error: %v", err)
            break
        }
        
        // Broadcast to other clients
        s.broadcastToCollaborators(msg)
    }
}
```

---

## Testing Strategy

### 1. Unit Tests

```typescript
// __tests__/syncEngine.test.ts
import { describe, it, expect } from 'vitest'
import { useSyncEngine } from '../hooks/useSyncEngine'

describe('SyncEngine', () => {
  it('should convert YAML to visual nodes', () => {
    const { syncYamlToVisual } = useSyncEngine()
    
    const yaml = `
name: test
on: cli.manual
steps:
  - id: echo
    use: core.echo
    with:
      text: hello
`
    
    const result = syncYamlToVisual(yaml)
    expect(result.nodes).toHaveLength(1)
    expect(result.nodes[0].data.id).toBe('echo')
  })
})
```

### 2. Integration Tests

```go
// Test the backend API
func TestVisualEditorAPI(t *testing.T) {
    server := NewTestServer()
    defer server.Close()
    
    // Test parse endpoint
    yaml := `name: test
on: cli.manual
steps:
  - id: echo
    use: core.echo
    with:
      text: hello`
    
    req := map[string]string{"yaml": yaml}
    resp := postJSON(server.URL+"/api/v1/visual/parse", req)
    
    var result VisualFlow
    json.Unmarshal(resp, &result)
    
    assert.Equal(t, 1, len(result.Nodes))
    assert.Equal(t, "echo", result.Nodes[0].ID)
}
```

---

## Deployment

### 1. Frontend Build

```bash
# Build for production
npm run build

# Serve static files from BeemFlow server
# Copy dist/ to BeemFlow's static assets
cp -r dist/* /path/to/beemflow/static/visual-editor/
```

### 2. Backend Integration

```go
// Serve static files
func (s *Server) setupStaticRoutes() {
    s.router.PathPrefix("/visual-editor/").Handler(
        http.StripPrefix("/visual-editor/", 
            http.FileServer(http.Dir("./static/visual-editor/"))))
}
```

---

## Performance Considerations

### 1. Optimize Large Flows

```typescript
// Use React Flow's virtualization
import { ReactFlowProvider } from '@reactflow/core'

const LargeFlowCanvas = () => {
  return (
    <ReactFlowProvider>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeOrigin={[0.5, 0.5]}
        // Enable virtualization for large flows
        onlyRenderVisibleElements={true}
        // Optimize re-renders
        nodesDraggable={true}
        nodesConnectable={true}
      />
    </ReactFlowProvider>
  )
}
```

### 2. Debounce Sync Operations

```typescript
// Debounce sync to prevent excessive API calls
const debouncedSync = useMemo(
  () => debounce((nodes: Node[], edges: Edge[]) => {
    const yaml = syncVisualToYaml(nodes, edges)
    setYaml(yaml)
  }, 300),
  [syncVisualToYaml]
)
```

---

This implementation guide provides a solid foundation for building the BeemFlow Visual Editor. Start with the proof of concept and gradually add more sophisticated features as you validate the approach with users.

The key is to maintain the bidirectional sync between visual and YAML representations while keeping the interface clean and intuitive. Focus on the core user experience first, then add advanced features like collaboration and complex node types.

**Happy coding! 🚀**
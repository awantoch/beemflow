# BeemFlow Visual Editor

> **Ultra-simple visual workflow editor powered by BeemFlow's Go runtime compiled to WASM**

A minimal, elegant visual editor that provides perfect bidirectional synchronization between YAML and visual drag-and-drop editing. Built with Go WASM for maximum code reuse and minimal dependencies.

## 🎯 Key Features

- **Perfect Sync**: Real-time bidirectional synchronization between YAML and visual modes
- **Zero Backend**: BeemFlow's Go runtime compiled to WASM runs entirely in the browser
- **Minimal Dependencies**: Just React Flow + Monaco Editor + BeemFlow WASM
- **Code Reuse**: 100% reuse of BeemFlow's existing parsing, validation, and graph generation
- **Offline Capable**: Works without internet connection after initial load
- **Fast**: Instant validation and parsing with no network latency

## 🚀 Quick Start

### Prerequisites

- Go 1.21+ (for building WASM)
- Node.js 18+ (for frontend)

### Setup

1. **Clone and navigate to the visual editor**:
```bash
cd visual-editor
```

2. **Install frontend dependencies**:
```bash
npm install
```

3. **Build the WASM module**:
```bash
npm run build:wasm
```

4. **Start development server**:
```bash
npm run dev
```

5. **Open your browser**:
Navigate to `http://localhost:3000`

### Build for Production

```bash
# Build both WASM and frontend
npm run build:all

# Or build separately
npm run build:wasm  # Build WASM module
npm run build       # Build frontend
```

## 📁 Project Structure

```
visual-editor/
├── wasm/                    # Go WASM module
│   ├── main.go             # WASM entry point & exports
│   ├── go.mod              # Points to parent BeemFlow
│   └── build.sh            # WASM build script
├── src/
│   ├── hooks/
│   │   └── useBeemFlow.ts  # WASM integration hook
│   ├── components/
│   │   └── StepNode.tsx    # Visual step components
│   ├── App.tsx             # Main application
│   ├── main.tsx            # React entry point
│   └── index.css           # Minimal styling
├── public/                  # Generated WASM files
│   ├── main.wasm           # BeemFlow runtime (generated)
│   └── wasm_exec.js        # Go WASM support (generated)
└── package.json
```

## 🔧 How It Works

### WASM Integration

The Go runtime is compiled to WASM and exposes these functions to JavaScript:

```go
// Available WASM functions
beemflowParseYaml(yaml: string)      // Parse YAML to Flow object
beemflowValidateYaml(yaml: string)   // Validate flow syntax
beemflowYamlToVisual(yaml: string)   // Convert to visual nodes/edges
beemflowVisualToYaml(visual: object) // Convert back to YAML
beemflowGenerateMermaid(yaml: string) // Generate Mermaid diagram
```

### React Integration

The `useBeemFlow` hook provides a clean interface:

```typescript
const { 
  wasmLoaded,     // WASM module ready
  yamlToVisual,   // Convert YAML → visual
  visualToYaml,   // Convert visual → YAML  
  validateYaml,   // Validate syntax
} = useBeemFlow()
```

### Bidirectional Sync

Changes in either editor instantly sync to the other:

1. **YAML → Visual**: Parse YAML, convert to React Flow nodes/edges
2. **Visual → YAML**: Convert nodes/edges back to Flow struct, generate YAML
3. **Validation**: Live validation with error highlighting
4. **Debouncing**: Prevents excessive updates during rapid editing

## 🎨 Usage

### Edit Modes

- **Visual**: Drag-and-drop node editor
- **YAML**: Monaco-powered text editor  
- **Split**: Both editors side-by-side (default)

### Node Types

Visual nodes automatically adapt to step types:

- 📢 `core.echo` - Blue core tools
- 🌐 `http.*` - Red HTTP operations  
- 🤖 `openai.*` - Green AI services
- 🧠 `anthropic.*` - Orange AI services
- 💬 `slack.*` - Purple integrations
- 📱 `twilio.*` - Pink communications

### Real-time Features

- ✅ **Live Validation**: Syntax errors shown instantly
- 🔄 **Auto-sync**: Changes propagate between editors
- 📊 **Visual Feedback**: Node colors indicate step types
- 🎯 **Dependency Visualization**: Automatic edge creation

## 🛠 Development

### WASM Development

To modify the WASM integration:

1. Edit `wasm/main.go`
2. Run `npm run build:wasm`
3. Refresh browser

### Frontend Development  

The Vite dev server supports hot reload:

```bash
npm run dev
```

### Adding New Features

**New WASM Functions:**
1. Add function to `wasm/main.go`
2. Export with `js.Global().Set(...)`
3. Update `useBeemFlow.ts` TypeScript declarations

**New Visual Components:**
1. Create component in `src/components/`
2. Register in `nodeTypes` (App.tsx)
3. Update `yamlToVisual` conversion logic

## 📦 Deployment

### Static Hosting

The built app is a static SPA that can be deployed anywhere:

```bash
npm run build:all

# Deploy 'dist/' folder to any static host:
# - Vercel: vercel --prod dist/
# - Netlify: netlify deploy --prod --dir dist/
# - AWS S3: aws s3 sync dist/ s3://your-bucket/
```

### BeemFlow Integration

To embed in BeemFlow's existing HTTP server:

```go
// Serve visual editor as static files
func (s *Server) setupVisualEditorRoutes() {
    s.router.PathPrefix("/visual-editor/").Handler(
        http.StripPrefix("/visual-editor/", 
            http.FileServer(http.Dir("./visual-editor/dist/"))))
}
```

## 🔍 Troubleshooting

### WASM Build Issues

```bash
# Ensure Go is configured for WASM
go env GOOS GOARCH
# Should show: js wasm

# Clean build
cd wasm && rm -f ../public/main.wasm && ./build.sh
```

### Frontend Issues

```bash
# Clear node modules and reinstall
rm -rf node_modules package-lock.json
npm install

# Clear Vite cache
rm -rf node_modules/.vite
```

### Performance Issues

- WASM module is ~15MB (acceptable for workflow editing)
- First load includes WASM download
- Subsequent loads are instant (cached)
- Consider CDN deployment for faster global access

## 🚀 Benefits vs Traditional Approach

| Traditional (Separate Backend) | WASM Approach |
|-------------------------------|---------------|
| Duplicate parsing logic | 100% code reuse |
| Network latency | Instant operations |
| Complex API design | Simple function calls |
| Multiple deployments | Single static build |
| Backend maintenance | Zero backend needed |

## 📊 File Sizes

- **WASM Module**: ~15MB (BeemFlow runtime)
- **Frontend JS**: ~2MB (React + deps)
- **Total**: ~17MB initial download
- **Subsequent**: Instant (cached)

Perfect for workflow editing where the 15MB download provides the full BeemFlow runtime capabilities offline.

## 🤝 Contributing

1. Make changes to Go code in parent `../` directory
2. Rebuild WASM: `npm run build:wasm`  
3. Test in browser
4. Changes automatically inherit from BeemFlow updates

## 📄 License

Same as BeemFlow main project.
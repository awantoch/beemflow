# BeemFlow Visual Editor

A browser-based visual editor for BeemFlow workflows with bidirectional YAML ↔ Visual editing.

## Architecture

- **WASM Runtime**: BeemFlow's Go codebase compiled to WebAssembly (~15MB)
- **Frontend**: React + React Flow + Monaco Editor
- **Zero Backend**: Runs entirely in the browser with no server dependencies

## Quick Start

```bash
# Build and run the editor
make editor

# Or build for production
make editor-build
```

The editor will be available at `http://localhost:3000/editor`

## Features

- **Split View**: Edit visually and in YAML simultaneously
- **Instant Validation**: Real-time YAML validation using BeemFlow's native parser
- **Drag & Drop**: Visual workflow creation with automatic YAML generation
- **Code Reuse**: 100% of BeemFlow's parsing, validation, and graph generation logic
- **Offline Ready**: Works without internet after initial load

## Development

```bash
# Install dependencies
cd editor/web && npm install

# Start dev server
npm run dev

# Build WASM module
make editor/wasm/main.wasm
```

## File Structure

```
editor/
├── wasm/           # Go WASM module
│   ├── main.go     # WASM entry point
│   └── wasm_exec.js # Go WASM runtime
└── web/            # React frontend
    ├── src/
    │   ├── App.tsx           # Main editor component
    │   ├── hooks/useBeemFlow.ts # WASM integration
    │   └── components/StepNode.tsx # Visual nodes
    ├── package.json
    └── vite.config.ts
```
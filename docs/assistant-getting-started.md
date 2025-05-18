# BeemFlow Assistant: Getting Started

BeemFlow Assistant makes it easy to create, validate, and run AI-powered workflows interactively—via CLI, HTTP, MCP, or any LLM playground.

## 1. Use in Any LLM Playground
- Copy `assistant/system_prompt.md` as your system prompt.
- Paste your flow authoring prompt as the user message.
- The assistant will return valid BeemFlow YAML.

## 2. Use via CLI
```sh
flow assist --prompt "Draft a flow that fetches weather and emails me if it rains"
```
- Interactive mode: `flow assist`
- On accept, write to file or `$EDITOR`.

## 3. Use via HTTP
```sh
curl -X POST http://localhost:8080/assistant/chat \
  -H 'Content-Type: application/json' \
  -d '{"messages": ["Draft a flow that..."]}'
```

## 4. Use via MCP
```yaml
- id: draft
  use: mcp://your-host/beemflow.assistant
  with:
    messages:
      - role: user
        content: "I need a 3-step flow that..."
```

## 5. Use in Any UI
- Import the manifest (`assistant/manifest.json`) and system prompt into your own UI or tool.

---

For more, see `assistant/examples.md` and the [BeemFlow Spec](../beemflow_spec.md). 
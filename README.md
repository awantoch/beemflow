# BeemFlow 🚀

---

# Table of Contents
- [BeemFlow 🚀](#beemflow-)
- [Table of Contents](#table-of-contents)
  - [What is BeemFlow?](#what-is-beemflow)
  - [Quickstart](#quickstart)
  - [Core Concepts](#core-concepts)
  - [Protocol-Agnostic Interface](#protocol-agnostic-interface)
  - [Real-World Examples](#real-world-examples)
    - [Hello World](#hello-world)
    - [Fetch and Summarize](#fetch-and-summarize)
    - [Parallel OpenAI (Fanout/Fanin)](#parallel-openai-fanoutfanin)
  - [Extending BeemFlow](#extending-beemflow)
  - [Project Layout](#project-layout)
  - [FAQ](#faq)
  - [Contributing \& Community](#contributing--community)
  - [Full Protocol \& Spec](#full-protocol--spec)
  - [Beemflow MCP Registry Integration](#beemflow-mcp-registry-integration)
    - [Multi-Registry Support](#multi-registry-support)
    - [Namespacing and Registry-Qualified Names](#namespacing-and-registry-qualified-names)
    - [Configuration](#configuration)
    - [CLI Usage](#cli-usage)
    - [Adding More Registries](#adding-more-registries)
    - [Migration Notes](#migration-notes)
  - [Registries: Curated vs Local](#registries-curated-vs-local)

---

## What is BeemFlow?

BeemFlow is an open protocol and runtime for AI-powered, event-driven automations. It provides a **protocol-agnostic, consistent interface** for flows and tools—CLI, HTTP, and MCP clients all speak the same language. Whether you're running a flow, discovering tools, or integrating with LLMs, you use the same concepts and API surface everywhere.

- **Text-first:** Write, share, and run workflows in YAML.
- **Interoperable:** Local, HTTP, and MCP tools are all available in a single, LLM-native registry.
- **Composable:** Chain tools, orchestrate workflows, and expose flows as tools for LLMs and clients.
- **Extensible:** Add new tools or adapters with zero boilerplate.

**Registry Resolution:** By default, BeemFlow uses `registry/index.json` if present. If not, it falls back to the public hub at `https://hub.beemflow.com/index.json`. You can override this with the `BEEMFLOW_REGISTRY` environment variable.

---

## Quickstart

1. **Clone the repo:**
   ```bash
   git clone https://github.com/awantoch/beemflow.git
   cd beemflow
   ```
2. **Run an example flow:**
   ```bash
   flow run hello
   # or add your OPENAI_API_KEY to .env and try another example:
   flow run fetch_and_summarize
   flow run parallel_openai
   ```
3. **Switch to NATS event bus:**
   ```bash
   cat <<EOF > flow.config.json
   { "event": { "driver":"nats", "url":"nats://..." } }
   EOF
   flow run yourflow  # now uses your NATS server
   ```
4. **Visualize a flow locally:**  
   ```bash
   flow graph <flow_file>
   ```
   This outputs the flow as Mermaid syntax (engineered for easy sharing).
5. **Run HTTP server with interactive UI:**  
   ```bash
   flow serve --port 8080
   ```
   Then open `http://localhost:8080` to select and visualize flows interactively.

---

## Core Concepts

- **Flows:** YAML files that define event-driven automations as a sequence of steps.
- **Steps:** Each step calls a tool (local, HTTP, or MCP) with inputs and produces outputs.
- **Adapters:** Pluggable integrations for HTTP APIs, LLMs, MCP servers, and custom logic.
- **Registry:** All tools—local manifests, MCP endpoints, remote registries—are auto-discovered and available in a single, LLM-native registry.
- **Protocol-Agnostic Interface:** Manage flows and tools the same way via CLI, HTTP, or MCP. Everything is interoperable and consistent.

---

## Protocol-Agnostic Interface

BeemFlow exposes a **consistent, protocol-agnostic interface** for running, managing, and introspecting flows and tools. Whether you use the CLI, HTTP API, or MCP protocol, you:
- List, run, and inspect flows
- Resume paused flows (durable waits)
- Validate and test flows
- Discover and call tools (from any source)
- Interact with the assistant for LLM-driven flow authoring

**See the [Full Protocol & Spec](docs/beemflow_spec.md) for canonical details, endpoints, and request/response formats.**

---

## Real-World Examples

### Hello World
```yaml
name: hello
on: cli.manual
steps:
  - id: greet
    use: core.echo
    with:
      text: "Hello, BeemFlow!"
  - id: print
    use: core.echo
    with:
      text: "{{.outputs.greet.text}}"
```
Run it:
```bash
flow run hello
```

### Fetch and Summarize
```yaml
name: fetch_and_summarize
on: cli.manual
steps:
  - id: fetch
    use: http.fetch
    with:
      url: "https://en.wikipedia.org/api/rest_v1/page/summary/Artificial_intelligence"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text in 3 bullet points."
        - role: user
          content: "{{.outputs.fetch.body}}"
  - id: print
    use: core.echo
    with:
      text: "{{.outputs.summarize.choices[0].message.content}}"
```
Run it:
```bash
flow run fetch_and_summarize
```

### Parallel OpenAI (Fanout/Fanin)
```yaml
name: parallel_openai
on: cli.manual
steps:
  - id: fanout
    parallel: true
    steps:
      - id: chat1
        use: openai.chat_completion
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "Prompt 1"
      - id: chat2
        use: openai.chat_completion
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "Prompt 2"
  - id: combine
    depends_on: [fanout]
    use: core.echo
    with:
      text: |
        Combined responses:\n
        - chat1: {{.outputs.fanout.chat1.choices[0].message.content}}
        - chat2: {{.outputs.fanout.chat2.choices[0].message.content}}
```
Run it:
```bash
flow run parallel_openai
```

---

## Extending BeemFlow

- **Add a local tool:** Use `flow mcp install <registry>:<tool>` or add entries to `.beemflow/registry.json`.
- **Add an MCP server:** Use `flow mcp install <registry>:<server>` or configure in `.beemflow/registry.json`.
- **Add a remote tool:** Reference a remote registry or GitHub manifest.
- **Write a custom adapter:** Implement the `Adapter` interface in Go.

All tools are auto-discovered and available in the registry, ready for use in flows, CLI, HTTP, MCP, or LLMs.

---

## Project Layout

```
my-beemflow/
├── flows/                 # .flow.yaml files
├── .beemflow/             # local registry and runtime state
├── flow.config.json       # backend & registry settings
└── README.md              # 👈 You're here
```

---

## FAQ

**Q: What's the difference between local, HTTP, and MCP tools?**
A: All are available in the same registry and can be used interchangeably in flows. Local tools are static manifests, HTTP tools are described by endpoint, and MCP tools are auto-discovered from MCP servers.

**Q: How do I override or extend the registry?**
A: Add a local manifest or MCP server config. You can shadow, extend, or remix tools without forking or duplicating JSON.

**Q: Can I host my own registry?**
A: Yes! Anyone can host a registry (even on a static website). BeemFlow comes with a default open registry out of the box, but you can add or override as needed.

**Q: How do I swap from memory to NATS for the event bus?**
A: Just add an `event` block to your `flow.config.json`:
```jsonc
{
  "event": {
    "driver": "nats",
    "url": "nats://user:pass@your-nats-host:4222"
  }
}
```

---

## Contributing & Community

BeemFlow is 100% open. We need YOU:
- Shape the spec
- Build adapters & UIs
- Share and remix flows
- Launch a SaaS or plugin on top

🌐 GitHub: https://github.com/awantoch/beemflow  
💬 Discord: https://discord.gg/your-invite  
📚 Docs: https://beemflow.com/docs

---

## Full Protocol & Spec

For the canonical, LLM-ingestible protocol, YAML grammar, API endpoints, and advanced examples, see:

👉 [docs/beemflow_spec.md](docs/beemflow_spec.md)

# Beemflow MCP Registry Integration

## Multi-Registry Support

Beemflow now supports multiple MCP registries (e.g., Smithery, local/unified) via a unified interface. You can:
- Search, list, and install MCP servers from all configured registries.
- Add new registries via environment variables or config.

## Namespacing and Registry-Qualified Names

When multiple registries are enabled, tool/server names are qualified by their registry:
- Format: `<registry>:<name>` (e.g., `smithery:airtable`, `local:mytool`)
- All CLI and API output includes a `REGISTRY` column.
- If a name is ambiguous (exists in more than one registry), you must specify the qualified name.
- If only one match exists, you can use the unqualified name for convenience.

### Example CLI Output

```
flow mcp list

REGISTRY   NAME         DESCRIPTION         KIND      ENDPOINT
smithery   airtable     Airtable MCP API   mcp_server  ...
local      mytool       My Local Tool      mcp_server  ...
```
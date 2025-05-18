# BeemFlow: Recursive & Human-in-the-Loop Flows

BeemFlow supports advanced patterns like flows that generate and execute other flows, and human-in-the-loop approval. Here's how to do it:

---

## 1. Recursive Flow Generation & Execution

```yaml
steps:
  - id: make
    use: beemflow.assistant
    with:
      messages:
        - role: user
          content: "Draft a flow that echoes 'hello world'"
  - id: run
    use: flow.execute
    with:
      flow_spec: "{{.outputs.make.draft}}"
      event: {}
```

- The assistant generates a new flow as YAML.
- `flow.execute` validates and runs it inline.

---

## 2. Human-in-the-Loop Approval

```yaml
steps:
  - id: draft
    use: beemflow.assistant
    with:
      messages:
        - role: user
          content: "Draft a Slack message to #alerts"
  - id: approve
    use: human.approval
    with:
      content: "Approve this message?"
      draft: "{{.outputs.draft.draft}}"
  - id: send
    use: slack.send
    with:
      channel: "#alerts"
      text: "{{.outputs.draft.draft}}"
```

- The assistant drafts a message.
- A human approves it before sending.

---

## 3. Testing & Validation
- Use `flow assist` or `/assistant/chat` to interactively build and validate these flows.
- Use `/runs/inline` or `flow.execute` to run them inline.

For more, see `assistant/examples.md` and the [BeemFlow Spec](../beemflow_spec.md). 
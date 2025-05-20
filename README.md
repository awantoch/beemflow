# BeemFlow

> **GitHub Actions for every business process — text-first, AI-native, open-source.**

BeemFlow is a **workflow runtime, protocol, and global tool registry** for the age of LLM co-workers.
Write a single YAML file → run it locally, over REST, or through the Model Context Protocol (MCP). The same spec powers the BeemFlow agency, SaaS, and acquisition flywheel—now you can build on it too.

---

## Table of Contents
- [BeemFlow](#beemflow)
  - [Table of Contents](#table-of-contents)
  - [Why BeemFlow?](#why-beemflow)
  - [Getting Started: BeemFlow by Example](#getting-started-beemflow-by-example)
    - [🟢 Example 1: Hello, World!](#-example-1-hello-world)
    - [🌐 Example 2: Fetch \& Summarize (LLM + HTTP)](#-example-2-fetch--summarize-llm--http)
  - [Workflow Gallery (Real-World Scenarios)](#workflow-gallery-real-world-scenarios)
    - [⚡ Parallel LLMs (Fan-out and Combine)](#-parallel-llms-fan-out-and-combine)
    - [🧑‍💼 Human-in-the-Loop Approval (MCP + Twilio SMS)](#-human-in-the-loop-approval-mcp--twilio-sms)
    - [🚀 Marketing Agent (LLM + Socials + Slack Approval)](#-marketing-agent-llm--socials--slack-approval)
    - [1. "CFO in a Box" – Daily 1-Slide Cash Report](#1-cfo-in-a-box--daily-1-slide-cash-report)
    - [2. E-Commerce Autopilot – Dynamic Pricing \& Ads](#2-e-commerce-autopilot--dynamic-pricing--ads)
    - [3. Invoice Chaser – Recover Aged AR in \< 24 h](#3-invoice-chaser--recover-aged-ar-in--24-h)
  - [Anatomy of a Flow](#anatomy-of-a-flow)
  - [Registry \& Tool Resolution](#registry--tool-resolution)
  - [CLI • HTTP • MCP — One Brain](#cli--http--mcp--one-brain)
  - [Extending BeemFlow](#extending-beemflow)
  - [Architecture](#architecture)
  - [Security \& Secrets](#security--secrets)
  - [Roadmap](#roadmap)
  - [Contributing](#contributing)
  - [License](#license)

---

## Why BeemFlow?

| Legacy "No-Code" | **BeemFlow** |
|------------------|--------------|
| Drag-and-drop UIs that break at scale | **Plain-text YAML** — diff-able, version-controlled, LLM-parseable |
| Opaque SaaS black boxes | **Open runtime** + plug-in adapters |
| Human glue work | **LLM prompts are first-class** – AI is the default worker |
| Multiple brittle dashboards | **One spec → one run → one audit trail** |
| Vendor lock-in | **Protocol-agnostic**: CLI, REST, MCP, library |

---

## Getting Started: BeemFlow by Example

**From "Hello, World!" to real-world automations. Each example is a real, runnable YAML file.**

---

### 🟢 Example 1: Hello, World!

**What it does:** Runs two steps, each echoing a message. Shows how outputs can be reused.

```yaml
# hello.flow.yaml
name: hello
on: cli.manual
steps:
  - id: greet
    use: core.echo
    with:
      text: "Hello, world, I'm BeemFlow!"
  - id: greet_again
    use: core.echo
    with:
      text: "Aaand once more: {{.greet.text}}"
```
```bash
flow run hello.flow.yaml
```
**Why it's powerful:**
- Shows how easy it is to pass data between steps.
- Demonstrates BeemFlow's text-first, LLM-friendly approach.

**What happens?**
- BeemFlow runs each step in order, passing outputs between them. You'll see both greetings printed.

---

### 🌐 Example 2: Fetch & Summarize (LLM + HTTP)

**What it does:** Fetches a web page, summarizes it with an LLM, and prints the result.

```yaml
# summarize.flow.yaml
name: fetch_and_summarize
on: cli.manual
vars:
  fetch_url: "https://en.wikipedia.org/wiki/Artificial_intelligence"
steps:
  - id: fetch_page
    use: http.fetch
    with:
      url: "{{.vars.fetch_url}}"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following web page in 3 bullets."
        - role: user
          content: "{{.fetch_page.body}}"
  - id: print
    use: core.echo
    with:
      text: "Summary: {{(index .summarize.choices 0).message.content}}"
```
```bash
flow run summarize.flow.yaml
```
**Why it's powerful:**
- Mixes HTTP, LLMs, and templating in one YAML.
- Shows how to use variables and step outputs.

**What happens?**
- BeemFlow fetches a web page, asks an LLM to summarize it, and prints the summary.

---

**Next Steps:**
- Try editing these flows or adding your own steps.
- Explore the Workflow Gallery below for more advanced, real-world automations.
- See [SPEC.md](./docs/SPEC.md) for the full grammar.

---

## Workflow Gallery (Real-World Scenarios)

Explore real-world automations, from parallel LLMs to human-in-the-loop and multi-channel marketing agents. Each example is detailed and ready to run.

---

### ⚡ Parallel LLMs (Fan-out and Combine)

**What it does:** Runs two LLM prompts in parallel, then combines their answers.

```yaml
# parallel.flow.yaml
name: parallel_facts
on: cli.manual
vars:
  prompt1: "Give me a fun fact about the Moon."
  prompt2: "Give me a fun fact about the Ocean."
steps:
  - id: fanout
    parallel: true
    steps:
      - id: moon_fact
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: user
              content: "{{.vars.prompt1}}"
      - id: ocean_fact
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: user
              content: "{{.vars.prompt2}}"
  - id: combine
    depends_on: [fanout]
    use: core.echo
    with:
      text: |
        🌕 Moon: {{(index .outputs.fanout.moon_fact.choices 0).message.content}}
        🌊 Ocean: {{(index .outputs.fanout.ocean_fact.choices 0).message.content}}
```
```bash
flow run parallel.flow.yaml
```
**Why it's powerful:**
- Effortless parallelism, LLM orchestration, and output composition.
- Shows how to use `parallel: true` and combine outputs.

**What happens?**
- BeemFlow runs two LLM prompts in parallel, then combines and prints their answers.

---

### 🧑‍💼 Human-in-the-Loop Approval (MCP + Twilio SMS)

**What it does:** Drafts a message, sends it for human approval via SMS (using an MCP tool), and acts on the reply.

```yaml
# human_approval.flow.yaml
name: human_approval
on: cli.manual
vars:
  phone_number: "+15551234567"  # <-- Replace with your test number
  approval_token: "demo-approval-123"
steps:
  - id: draft_message
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Draft a short, friendly reminder for a team meeting at 3pm."
  - id: send_sms
    use: mcp://twilio/send_sms
    with:
      to: "{{.vars.phone_number}}"
      body: |
        {{(index .draft_message.choices 0).message.content}}
        Reply YES to approve, NO to reject.
      token: "{{.vars.approval_token}}"
  - id: wait_for_approval
    await_event:
      source: twilio
      match:
        token: "{{.vars.approval_token}}"
      timeout: 1h
  - id: check_approval
    if: "{{.event.body | toLower | trim == 'yes'}}"
    use: core.echo
    with:
      text: "✅ Approved! Message sent."
  - id: check_rejection
    if: "{{.event.body | toLower | trim == 'no'}}"
    use: core.echo
    with:
      text: "❌ Rejected by human."
```
```bash
flow run human_approval.flow.yaml
```
**Why it's powerful:**
- Brings in external tools (MCP), durable waits, and human-in-the-loop automation.
- Shows how to use `await_event` and conditional logic.

**What happens?**
- The flow sends an SMS for approval.
- It pauses until a reply is received (via webhook or manual event).
- When the human replies, the flow resumes and prints the result.

---

### 🚀 Marketing Agent (LLM + Socials + Slack Approval)

**What it does:**
- Takes a feature update as input.
- Uses LLM(s) to generate content for Twitter, LinkedIn, and a blog post.
- Sends the drafts to a Slack channel for team review/approval.
- Waits for Slack feedback/approval before posting to the socials (simulated as echo steps for safety, but can be swapped for real posting tools).

```yaml
# marketing_agent.flow.yaml
name: marketing_agent
on: cli.manual
vars:
  feature_update: "BeemFlow now supports human-in-the-loop approvals via SMS and Slack!"
  approval_token: "approved!"
  slack_channel: "#marketing"
steps:
  - id: generate_content
    parallel: true
    steps:
      - id: tweet
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: system
              content: "Write a catchy tweet announcing this product update: '{{.vars.feature_update}}'"
      - id: linkedin
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: system
              content: "Write a LinkedIn post (max 300 words) for this update: '{{.vars.feature_update}}'"
      - id: blog
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: system
              content: "Write a short blog post (max 500 words) about: '{{.vars.feature_update}}'"
  - id: send_to_slack
    use: mcp://slack/chat.postMessage
    with:
      channel: "{{.vars.slack_channel}}"
      text: |
        :mega: *Feature Update Drafts for Review*
        *Tweet:* {{(index .outputs.generate_content.tweet.choices 0).message.content}}
        *LinkedIn:* {{(index .outputs.generate_content.linkedin.choices 0).message.content}}
        *Blog:* {{(index .outputs.generate_content.blog.choices 0).message.content}}
        
        Reply with 'approve' to post, or 'edit: ...' to suggest changes.
      token: "{{.vars.approval_token}}"
  - id: wait_for_slack_approval
    await_event:
      source: slack
      match:
        token: "{{.vars.approval_token}}"
      timeout: 2h
  - id: handle_edits
    if: "{{.event.text | toLower | hasPrefix 'edit:'}}"
    use: core.echo
    with:
      text: "Edits requested: {{.event.text}} (flow would branch to editing here)"
  - id: post_to_socials
    if: "{{.event.text | toLower | trim == 'approve'}}"
    parallel: true
    steps:
      - id: post_tweet
        use: core.echo  # Replace with mcp://twitter/post for real posting
        with:
          text: "[POSTED to Twitter]: {{(index .outputs.generate_content.tweet.choices 0).message.content}}"
      - id: post_linkedin
        use: core.echo  # Replace with mcp://linkedin/post for real posting
        with:
          text: "[POSTED to LinkedIn]: {{(index .outputs.generate_content.linkedin.choices 0).message.content}}"
      - id: post_blog
        use: core.echo  # Replace with mcp://blog/post for real posting
        with:
          text: "[POSTED to Blog]: {{(index .outputs.generate_content.blog.choices 0).message.content}}"
```
```bash
flow run marketing_agent.flow.yaml
```
**Why it's powerful:**
- Orchestrates multiple LLMs, parallel content generation, and human-in-the-loop review across channels.
- Shows how to combine parallelism, templating, and event-driven waits in a real-world marketing workflow.

**What happens?**
- The flow generates content for each channel in parallel.
- It sends all drafts to Slack for review.
- It waits for a human to reply 'approve' or 'edit: ...'.
- If approved, it posts to all channels (simulated here with echo steps).
- If edits are requested, it echoes the request (could branch to a revision flow).

---

### 1. "CFO in a Box" – Daily 1-Slide Cash Report

**What it does:**
- Pulls balances from Stripe and QuickBooks.
- Analyzes and summarizes cash/AR with an LLM.
- Converts the summary to a PowerPoint slide.
- Sends the slide to Slack.

```yaml
name: cfo_daily_cash
on: schedule.cron
cron: "0 7 * * *"          # 07:00 every day

vars:
  ALERT_THRESHOLD: 20000

steps:
  - id: pull_stripe
    use: stripe.balance.retrieve
    with: { api_key: "{{.secrets.STRIPE_KEY}}" }

  - id: pull_qbo
    use: quickbooks.reports.balanceSheet
    with: { token: "{{.secrets.QBO_TOKEN}}" }

  - id: analyze
    use: openai.chat_completion
    with:
      model: gpt-4o
      messages:
        - role: system
          content: |
            Combine the Stripe and QuickBooks JSON below.
            1. Report total cash & AR.
            2. If cash < ${{vars.ALERT_THRESHOLD}}, add ⚠️.
            3. Format as a single PowerPoint slide in Markdown.
        - role: user
          content: |
            Stripe: {{.outputs.pull_stripe}}
            QuickBooks: {{.outputs.pull_qbo}}

  - id: ppt
    use: cloudconvert.md_to_pptx
    with:
      markdown: "{{.outputs.analyze.choices[0].message.content}}"

  - id: send
    use: slack.files.upload
    with:
      token: "{{.secrets.SLACK_TOKEN}}"
      channels: ["#finance"]
      file: "{{.outputs.ppt.file_url}}"
      title: "Daily Cash Snapshot"
```
```bash
flow run cfo_daily_cash.flow.yaml
```
**Why it's powerful:**
- Shows multi-source data, LLM analysis, file conversion, and Slack integration in one flow.

**What happens?**
- Pulls balances, analyzes, generates a slide, and sends it to Slack—automatically, every morning.

---

### 2. E-Commerce Autopilot – Dynamic Pricing & Ads

**What it does:**
- Scrapes competitor prices.
- Updates Shopify product prices based on margin and competitor data.
- Adjusts Google Ads campaigns based on price changes.

```yaml
name: ecommerce_autopilot
on: schedule.interval
every: "1h"

vars:
  MIN_MARGIN_PCT: 20

steps:
  - id: scrape_prices
    use: browserless.scrape
    with:
      url: "https://competitor.com/product/{{.event.sku}}"
      selector: ".price"
      format: json

  - id: update_shopify
    use: shopify.product.updatePrice
    with:
      api_key: "{{.secrets.SHOPIFY_KEY}}"
      product_id: "{{.event.product_id}}"
      new_price: |
        {{ math.max(
             event.cost * (1 + vars.MIN_MARGIN_PCT/100),
             outputs.scrape_prices.price * 0.98
           ) }}

  - id: adjust_ads
    use: googleads.campaigns.update
    with:
      token: "{{.secrets.GADS_TOKEN}}"
      campaign_id: "{{.event.campaign_id}}"
      target_roas: |
        {{ 1.3 if outputs.update_shopify.changed else 1.1 }}
```
```bash
flow run ecommerce_autopilot.flow.yaml
```
**Why it's powerful:**
- Shows event-driven automation, dynamic pricing, and multi-system orchestration.

**What happens?**
- Scrapes competitor prices, updates your store, and tunes ads—on autopilot, every hour.

---

### 3. Invoice Chaser – Recover Aged AR in < 24 h

**What it does:**
- Fetches overdue invoices from QuickBooks.
- Sends reminder emails and waits 24h.
- Checks if paid; if not, escalates with a Twilio SMS.

```yaml
name: invoice_chaser
on: schedule.cron
cron: "0 9 * * 1-5"  # every weekday 09:00

steps:
  - id: fetch_overdue
    use: quickbooks.reports.aging
    with: { token: "{{.secrets.QBO_TOKEN}}" }

  - id: foreach_invoice
    foreach: "{{.outputs.fetch_overdue.invoices}}"
    as: inv
    do:
      - id: email_first
        use: postmark.email.send
        with:
          api_key: "{{.secrets.EMAIL_KEY}}"
          to: "{{.inv.customer_email}}"
          template: "overdue_reminder"
          vars: { days: "{{.inv.days_overdue}}", amount: "{{.inv.balance}}" }

      - id: wait_24h
        wait: { hours: 24 }

      - id: check_paid
        use: quickbooks.invoice.get
        with: { id: "{{.inv.id}}", token: "{{.secrets.QBO_TOKEN}}" }

      - id: escalate
        if: "{{.outputs.check_paid.status != 'Paid'}}"
        use: twilio.sms.send
        with:
          sid: "{{.secrets.TWILIO_SID}}"
          auth: "{{.secrets.TWILIO_AUTH}}"
          to: "{{.inv.customer_phone}}"
          body: "Friendly nudge: Invoice #{{.inv.id}} is now {{.inv.days_overdue+1}} days overdue."
```
```bash
flow run invoice_chaser.flow.yaml
```
**Why it's powerful:**
- Shows foreach loops, waits, conditional logic, and escalation.

**What happens?**
- Chases overdue invoices, escalates if unpaid, and automates the whole AR follow-up process.

---

**Next Steps:**
- Try running or editing any of these flows.
- Build your own automations by remixing steps.
- See [SPEC.md](./docs/SPEC.md) for the full YAML grammar and advanced features.

---

## Anatomy of a Flow

```yaml
name: fetch_and_summarize
on: cli.manual
vars:
  TOPIC: "Artificial_intelligence"
steps:
  - id: fetch
    use: http.fetch
    with: { url: "https://en.wikipedia.org/wiki/{{.vars.TOPIC}}" }

  - id: summarize
    use: openai.chat_completion
    with:
      model: gpt-4o
      messages:
        - role: system
          content: "Summarize the following text in 3 bullets."
        - role: user
          content: "{{.outputs.fetch.body}}"

  - id: announce
    use: slack.chat.postMessage
    with:
      channel: "#ai-updates"
      text: "{{.outputs.summarize.choices[0].message.content}}"
```

✨ **Templating:** `{{…}}` gives you outputs, vars, secrets, helper funcs.
⏳ **Durable waits:** `await_event` pauses until external approval / webhook.
⚡ **Parallelism & retries:** `parallel: true` blocks and `retry:` back-offs.
🔄 **Error handling:** `catch:` block processes failures.

Full grammar ➜ [SPEC.md](./docs/SPEC.md).

---

## Registry & Tool Resolution

Priority:

1. `$BEEMFLOW_REGISTRY`
2. `registry/index.json`
3. `https://hub.beemflow.com/index.json`

Tools can be qualified (`smithery:airtable`) when ambiguous.

---

## CLI • HTTP • MCP — One Brain

| Action        | CLI                 | HTTP            | MCP            |
|---------------|---------------------|-----------------|----------------|
| Validate flow | `flow lint file`    | `POST /validate`| `validateFlow` |
| Run flow      | `flow run hello`    | `POST /runs`    | `startRun`     |
| Status        | `flow status <id>`  | `GET /runs/{id}`| `getRun`       |
| Graph         | `flow graph file`   | `GET /graph`    | `graphFlow`    |

---

## Extending BeemFlow

- **Add a tool**: `flow mcp install registry:tool` or edit `.beemflow/registry.json`.
- **Custom adapter**: implement the `Adapter` interface in your own code.
- **Swap event bus**: set `"event.driver": "nats"` in `flow.config.json` or via `BEEMFLOW_EVENT_DRIVER=nats`.
- **LLM autopilot**: POST `/assistant/chat` with system prompt in [SPEC.md](./SPEC.md#14).

---

## Architecture

- Router & planner (DAG builder)
- Executor (persistent state, retries, awaits)
- Event bus (memory, NATS, Temporal future)
- Registry & adapters

---

## Security & Secrets

- Secrets from env, Vault, or MCP store: `{{.secrets.NAME}}`.
- HMAC-signed resume tokens for durable waits.
- SOC 2 Type II in progress; ISO 27001 roadmap next.

---

## Roadmap

- VS Code extension (YAML + Mermaid preview).
- Flow template gallery (`flow init payroll` etc.).
- Cron & Temporal adapters.
- Hot-reload adapters without downtime.
- OpenTelemetry metrics & traces.
- On-chain event bus (experimental).

---

## Contributing

```bash
git clone https://github.com/beemflow/beemflow
make dev
```

- **Code**: Go 1.22+, linted, tested.
- **Docs**: PRs welcome — every example is CI-verified.
- **Community**: Join <https://discord.gg/beemflow>.

---

## License

MIT — use it, remix it, ship it.
Commercial cloud & SLA on the way.

---

> "We're doing to Zapier what GitHub did to FTP—text-based, versioned, and supercharged by AI labor."
> Docs at <https://beemflow.com/docs> • Twitter: [@BeemFlow](https://twitter.com/beemflow)

# BeemFlow

> **GitHub Actions for every business process — text-first, AI-native, open-source.**

BeemFlow is a **workflow protocol, runtime, and global tool registry** for the age of LLM co-workers.
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
    - [💼 "CFO in a Box" – Daily 1-Slide Cash Report](#-cfo-in-a-box--daily-1-slide-cash-report)
    - [🛒 E-Commerce Autopilot – Dynamic Pricing \& Ads](#-e-commerce-autopilot--dynamic-pricing--ads)
    - [📬 Invoice Chaser – Recover Aged AR in \< 24 h](#-invoice-chaser--recover-aged-ar-in--24-h)
  - [Anatomy of a Flow](#anatomy-of-a-flow)
  - [Flows as Functions: Universal, Protocolized, and Language-Native](#flows-as-functions-universal-protocolized-and-language-native)
    - [YAML](#yaml)
    - [Go](#go)
    - [TypeScript](#typescript)
    - [Rust](#rust)
    - [Python](#python)
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
      text: "Aaand once more: {{ greet.text }}"
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
      url: "{{ fetch_url }}"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following web page in 3 bullets."
        - role: user
          content: "{{ fetch_page.body }}"
  - id: print
    use: core.echo
    with:
      text: "Summary: {{ summarize.choices.0.message.content }}"
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
              content: "{{ prompt1 }}"
      - id: ocean_fact
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: user
              content: "{{ prompt2 }}"
  - id: combine
    depends_on: [fanout]
    use: core.echo
    with:
      text: |
        🌕 Moon: {{ moon_fact.choices.0.message.content }}
        🌊 Ocean: {{ ocean_fact.choices.0.message.content }}
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
      to: "{{ phone_number }}"
      body: |
        {{ draft_message.choices.0.message.content }}
        Reply YES to approve, NO to reject.
      token: "{{ approval_token }}"
  - id: wait_for_approval
    await_event:
      source: twilio
      match:
        token: "{{ approval_token }}"
      timeout: 1h
  - id: check_approval
    if: "{{ event.body | toLower | trim == 'yes' }}"
    use: core.echo
    with:
      text: "✅ Approved! Message sent."
  - id: check_rejection
    if: "{{ event.body | toLower | trim == 'no' }}"
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
- Uses LLM(s) to generate content for X, LinkedIn, and a blog post.
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
              content: "Write a catchy tweet announcing this product update: '{{ feature_update }}'"
      - id: linkedin
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: system
              content: "Write a LinkedIn post (max 300 words) for this update: '{{ feature_update }}'"
      - id: blog
        use: openai.chat_completion
        with:
          model: "gpt-4o"
          messages:
            - role: system
              content: "Write a short blog post (max 500 words) about: '{{ feature_update }}'"
  - id: send_to_slack
    use: mcp://slack/chat.postMessage
    with:
      channel: "{{ slack_channel }}"
      text: |
        :mega: *Feature Update Drafts for Review*
        *Tweet:* {{ tweet.choices.0.message.content }}
        *LinkedIn:* {{ linkedin.choices.0.message.content }}
        *Blog:* {{ blog.choices.0.message.content }}
        
        Reply with 'approve' to post, or 'edit: ...' to suggest changes.
      token: "{{ approval_token }}"
  - id: wait_for_slack_approval
    await_event:
      source: slack
      match:
        token: "{{ approval_token }}"
      timeout: 2h
  - id: handle_edits
    if: "{{ event.text | toLower | hasPrefix 'edit:' }}"
    use: core.echo
    with:
      text: "Edits requested: {{ event.text }} (flow would branch to editing here)"
  - id: post_to_socials
    if: "{{ event.text | toLower | trim == 'approve' }}"
    parallel: true
    steps:
      - id: x_post
        use: mcp://x/post
        with:
          text: "[POSTED to X]: {{ tweet.choices.0.message.content }}"
      - id: post_linkedin
        use: mcp://linkedin/post
        with:
          text: "[POSTED to LinkedIn]: {{ linkedin.choices.0.message.content }}"
      - id: post_blog
        use: mcp://blog/post
        with:
          text: "[POSTED to Blog]: {{ blog.choices.0.message.content }}"
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

### 💼 "CFO in a Box" – Daily 1-Slide Cash Report

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
    with: { api_key: "{{ secrets.STRIPE_KEY }}" }

  - id: pull_qbo
    use: quickbooks.reports.balanceSheet
    with: { token: "{{ secrets.QBO_TOKEN }}" }

  - id: analyze
    use: openai.chat_completion
    with:
      model: gpt-4o
      messages:
        - role: system
          content: |
            Combine the Stripe and QuickBooks JSON below.
            1. Report total cash & AR.
            2. If cash < ${{ vars.ALERT_THRESHOLD }}, add ⚠️.
            3. Format as a single PowerPoint slide in Markdown.
        - role: user
          content: |
            Stripe: {{ pull_stripe }}
            QuickBooks: {{ pull_qbo }}

  - id: ppt
    use: cloudconvert.md_to_pptx
    with:
      markdown: "{{ analyze.choices.0.message.content }}"

  - id: send
    use: slack.files.upload
    with:
      token: "{{ secrets.SLACK_TOKEN }}"
      channels: ["#finance"]
      file: "{{ ppt.file_url }}"
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

### 🛒 E-Commerce Autopilot – Dynamic Pricing & Ads

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
      url: "https://competitor.com/product/{{ event.sku }}"
      selector: ".price"
      format: json

  - id: update_shopify
    use: shopify.product.updatePrice
    with:
      api_key: "{{ secrets.SHOPIFY_KEY }}"
      product_id: "{{ event.product_id }}"
      new_price: |
        {{ math.max(
             event.cost * (1 + vars.MIN_MARGIN_PCT/100),
             outputs.scrape_prices.price * 0.98
           ) }}

  - id: adjust_ads
    use: googleads.campaigns.update
    with:
      token: "{{ secrets.GADS_TOKEN }}"
      campaign_id: "{{ event.campaign_id }}"
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

### 📬 Invoice Chaser – Recover Aged AR in < 24 h

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
    with: { token: "{{ secrets.QBO_TOKEN }}" }

  - id: foreach_invoice
    foreach: "{{ fetch_overdue.invoices }}"
    as: inv
    do:
      - id: email_first
        use: postmark.email.send
        with:
          api_key: "{{ secrets.EMAIL_KEY }}"
          to: "{{ inv.customer_email }}"
          template: "overdue_reminder"
          vars: { days: "{{ inv.days_overdue }}", amount: "{{ inv.balance }}" }

      - id: wait_24h
        wait: { hours: 24 }

      - id: check_paid
        use: quickbooks.invoice.get
        with: { id: "{{ inv.id }}", token: "{{ secrets.QBO_TOKEN }}" }

      - id: escalate
        if: "{{ outputs.check_paid.status != 'Paid' }}"
        use: twilio.sms.send
        with:
          sid: "{{ secrets.TWILIO_SID }}"
          auth: "{{ secrets.TWILIO_AUTH }}"
          to: "{{ inv.customer_phone }}"
          body: "Friendly nudge: Invoice #{{ inv.id }} is now {{ inv.days_overdue+1 }} days overdue."
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
    with: { url: "https://en.wikipedia.org/wiki/{{ TOPIC }}" }

  - id: summarize
    use: openai.chat_completion
    with:
      model: gpt-4o
      messages:
        - role: system
          content: "Summarize the following text in 3 bullets."
        - role: user
          content: "{{ outputs.fetch.body }}"

  - id: announce
    use: slack.chat.postMessage
    with:
      channel: "#ai-updates"
      text: "{{ summarize.choices.0.message.content }}"
```

✨ **Templating:** `{{…}}` gives you outputs, vars, secrets, helper funcs.
⏳ **Durable waits:** `await_event` pauses until external approval / webhook.
⚡ **Parallelism & retries:** `parallel: true` blocks and `retry:` back-offs.
🔄 **Error handling:** `catch:` block processes failures.

Full grammar ➜ [SPEC.md](./docs/SPEC.md).

---

## Flows as Functions: Universal, Protocolized, and Language-Native

> **Define flows in YAML, or as native data structures in your favorite language. Compose, generate, and run flows as code—unlocking the power of "flows as functions" and true protocolization.**

The BeemFlow protocol is not just for YAML. You can define flows as native structs or objects in any language, and run them using the same universal grammar. This means you can build, compose, and share automations as code—making BeemFlow the most flexible, programmable workflow engine for both devs and non-devs.

Below, see the same flow defined in YAML, Go, TypeScript, Rust, and Python:

---

### YAML
```yaml
name: fetch_and_summarize
on: cli.manual
vars:
  URL: "https://en.wikipedia.org/wiki/Artificial_intelligence"
steps:
  - id: fetch
    use: http.fetch
    with:
      url: "{{ URL }}"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text in 3 bullet points."
        - role: user
          content: "{{ outputs.fetch.body }}"
  - id: print
    use: core.echo
    with:
      text: "{{ summarize.choices.0.message.content }}"
```

### Go
```go
package main

import (
  "fmt"
  "gopkg.in/yaml.v3"
  "net/http"
  "bytes"
  "io/ioutil"
  "github.com/awantoch/beemflow/model"
)

func main() {
  flow := model.Flow{
    Name: "fetch_and_summarize",
    On:   "cli.manual",
    Vars: map[string]interface{}{
      "URL": "https://en.wikipedia.org/wiki/Artificial_intelligence",
    },
    Steps: []model.Step{
      {
        ID:  "fetch",
        Use: "http.fetch",
        With: map[string]interface{}{
          "url": "{{ URL }}",
        },
      },
      {
        ID:  "summarize",
        Use: "openai.chat_completion",
        With: map[string]interface{}{
          "model": "gpt-4o",
          "messages": []interface{}{
            map[string]interface{}{ "role": "system", "content": "Summarize in 3 bullets." },
            map[string]interface{}{ "role": "user",   "content": "{{ outputs.fetch.body }}" },
          },
        },
      },
      {
        ID:  "print",
        Use: "core.echo",
        With: map[string]interface{}{
          "text": "{{ summarize.choices.0.message.content }}",
        },
      },
    },
  }

  out, err := yaml.Marshal(flow)
  if err != nil {
    panic(err)
  }
  fmt.Println(string(out))

  // --- Run the flow via HTTP ---
  // (Assume BeemFlow is running at http://localhost:3333)
  reqBody := []byte(`{"flow": "fetch_and_summarize", "event": {}}`)
  resp, err := http.Post("http://localhost:3333/runs", "application/json", bytes.NewBuffer(reqBody))
  if err != nil {
    panic(err)
  }
  defer resp.Body.Close()
  body, _ := ioutil.ReadAll(resp.Body)
  fmt.Println(string(body))
}
```

### TypeScript
```typescript
import * as yaml from 'js-yaml';
import fetch from 'node-fetch';

interface Flow {
  name: string;
  on: string | object;
  vars?: Record<string, any>;
  steps: Step[];
}

interface Step {
  id: string;
  use: string;
  with: Record<string, any>;
}

const flow: Flow = {
  name: "fetch_and_summarize",
  on: "cli.manual",
  vars: {
    URL: "https://en.wikipedia.org/wiki/Artificial_intelligence",
  },
  steps: [
    { id: "fetch",     use: "http.fetch",           with: { url: "{{ URL }}" } },
    { id: "summarize", use: "openai.chat_completion", with: {
        model: "gpt-4o",
        messages: [
          { role: "system", content: "Summarize in 3 bullets." },
          { role: "user",   content: "{{ outputs.fetch.body }}" },
        ],
      },
    },
    { id: "print", use: "core.echo", with: {
        text: "{{ summarize.choices.0.message.content }}",
      },
    },
  ],
};

console.log(yaml.dump(flow));

// --- Run the flow via HTTP ---
(async () => {
  const response = await fetch('http://localhost:3333/runs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ flow: flow, event: {} }),
  });
  const data = await response.text();
  console.log(data);
})();
```

### Rust
```rust
use serde::{Serialize, Deserialize};
use serde_yaml;
use std::collections::HashMap;
use reqwest::blocking::Client;

#[derive(Serialize, Deserialize)]
struct Flow {
    name: String,
    on: String,
    vars: Option<HashMap<String, serde_yaml::Value>>,
    steps: Vec<Step>,
}

#[derive(Serialize, Deserialize)]
struct Step {
    id: String,
    #[serde(rename = "use")]
    use_: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    with: Option<HashMap<String, serde_yaml::Value>>,
}

fn main() {
    let mut vars = HashMap::new();
    vars.insert("URL".into(), serde_yaml::Value::String("https://en.wikipedia.org/wiki/Artificial_intelligence".into()));

    let flow = Flow {
        name: "fetch_and_summarize".into(),
        on:   "cli.manual".into(),
        vars: Some(vars),
        steps: vec![
            Step {
                id:   "fetch".into(),
                use_: "http.fetch".into(),
                with: Some({
                    let mut m = HashMap::new();
                    m.insert("url".into(), serde_yaml::Value::String("{{ URL }}"))
                }),
            },
            Step {
                id:   "summarize".into(),
                use_: "openai.chat_completion".into(),
                with: Some({
                    let mut m = HashMap::new();
                    m.insert("model".into(), serde_yaml::Value::String("gpt-4o".into()));
                    m.insert("messages".into(), serde_yaml::Value::Sequence(vec![
                        serde_yaml::Value::Mapping({
                            let mut sys = serde_yaml::Mapping::new();
                            sys.insert(serde_yaml::Value::String("role".into()), serde_yaml::Value::String("system".into()));
                            sys.insert(serde_yaml::Value::String("content".into()), serde_yaml::Value::String("Summarize in 3 bullets.".into()));
                            sys
                        }),
                        serde_yaml::Value::Mapping({
                            let mut user = serde_yaml::Mapping::new();
                            user.insert(serde_yaml::Value::String("role".into()), serde_yaml::Value::String("user".into()));
                            user.insert(serde_yaml::Value::String("content".into()), serde_yaml::Value::String("{{ outputs.fetch.body }}".into()));
                            user
                        }),
                    ]));
                    m
                }),
            },
            Step {
                id:   "print".into(),
                use_: "core.echo".into(),
                with: Some({
                    let mut m = HashMap::new();
                    m.insert("text".into(), serde_yaml::Value::String("{{ summarize.choices.0.message.content }}".into()));
                    m
                }),
            },
        ],
    };

    println!("{}", serde_yaml::to_string(&flow).unwrap());

    // --- Run the flow via HTTP ---
    let client = Client::new();
    let body = serde_json::json!({ "flow": flow, "event": {} });
    let res = client.post("http://localhost:3333/runs")
        .json(&body)
        .send()
        .unwrap();
    println!("{}", res.text().unwrap());
}
```

### Python
```python
from dataclasses import dataclass, asdict
from typing import Any, Dict, List, Union
import requests
import yaml

@dataclass
class Step:
    id: str
    use: str
    with_: Dict[str, Any]

@dataclass
class Flow:
    name: str
    on: Union[str, dict, List[Union[str, dict]]]
    vars: Dict[str, Any]
    steps: List[Step]

flow = Flow(
    name="fetch_and_summarize",
    on="cli.manual",
    vars={"URL": "https://en.wikipedia.org/wiki/Artificial_intelligence"},
    steps=[
        Step("fetch",     "http.fetch",           {"url": "{{ URL }}"}),
        Step("summarize", "openai.chat_completion", {"model": "gpt-4o", "messages":[
            {"role":"system","content":"Summarize in 3 bullets."},
            {"role":"user",   "content":"{{ outputs.fetch.body }}" },
        ]}),
        Step("print",     "core.echo",            {"text":"{{ summarize.choices.0.message.content }}"}),
    ]
)

d = asdict(flow)
for s in d["steps"]:
    s["with"] = s.pop("with_")
print(yaml.dump(d))

# --- Run the flow via HTTP ---
resp = requests.post(
    "http://localhost:3333/runs",
    json={"flow": d, "event": {}},
)
print(resp.text)
```

---

> **BeemFlow: One protocol, infinite languages. Program the world.**

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
- **LLM autopilot**: POST `/assistant/chat` with system prompt in [SPEC.md](./docs/SPEC.md#14).

---

## Architecture

- Router & planner (DAG builder)
- Executor (persistent state, retries, awaits)
- Event bus (memory, NATS, Temporal future)
- Registry & adapters

---

## Security & Secrets

- Secrets from env, Vault, or MCP store: `{{ secrets.NAME }}`.
- HMAC-signed resume tokens for durable waits.
- SOC 2 Type II in progress; ISO 27001 roadmap next.

---

## Roadmap

- VS Code extension (YAML + Mermaid preview).
- Flow template gallery (`flow init payroll` etc.).
- Cron & Temporal adapters.
- Hot-reload adapters without downtime.
- On-chain event bus (experimental).

---

## Contributing

```bash
git clone https://github.com/beemflow/beemflow
make dev
```

- **Code**: Go 1.22+, linted, tested.
- **Docs**: PRs welcome — every example is CI-verified and BeemFlow-reviewed.
- **Community**: Join <https://discord.gg/beemflow>.

---

## License

MIT — use it, remix it, ship it.
Commercial cloud & SLA on the way.

---

> Docs at <https://docs.beemflow.com> • X: [@BeemFlow](https://X.com/beemflow)
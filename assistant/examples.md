# BeemFlow Assistant: Examples

## Basic Chat Example

**User:**
I need a flow that fetches weather data and sends me an email if it rains.

**Assistant:**
```yaml
steps:
  - id: get_weather
    use: http.get
    with:
      url: "https://api.weather.com/v3/weather/forecast"
  - id: check_rain
    use: core.if
    with:
      condition: "{{.outputs.get_weather.body.contains('rain')}}"
      then:
        - id: send_email
          use: email.send
          with:
            to: "me@example.com"
            subject: "Rain Alert!"
            body: "It's going to rain."
```

---

## Recursive Flow Example

**User:**
Generate a flow that creates another flow and runs it.

**Assistant:**
```yaml
steps:
  - id: make
    use: beemflow.assistant
    with:
      messages:
        - role: user
          content: "Make a flow that echoes 'hello world'"
  - id: run
    use: flow.execute
    with:
      flow_spec: "{{.outputs.make.draft}}"
      event: {}
```

---

## Human-in-the-Loop Example

**User:**
Draft a flow that requires human approval before sending a Slack message.

**Assistant:**
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
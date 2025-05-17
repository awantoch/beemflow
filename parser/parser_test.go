package parser

import (
	"os"
	"testing"

	"github.com/awantoch/beemflow/model"
)

const helloWorldFlow = `
name: hello
on: cli.manual
steps:
  - id: greet
    use: agent.llm.chat
    with:
      system: "Hey BeemFlow!"
      text: "Hello, world!"
  - id: print
    use: core.echo
    with:
      text: "{{greet.text}}"
`

const marketingBlastFlow = `name: launch_blast
on:
  - event: webhook.product_feature

vars:
  wait_between_polls: 30

steps:
  - id: search_docs
    use: docs.search
    with:
      query: "{{event.feature}}"
      top_k: 5

  - id: marketing_context
    use: agent.llm.summarize
    with:
      system: "You are product marketing."
      text: |
        ### Feature
        {{event.feature}}
        ### Docs
        {{search_docs.results | join("\n\n")}}
      max_tokens: 400

  - id: gen_copy
    use: agent.llm.function_call
    with:
      function_schema: |
        { "name": "mk_copy", "parameters": {
          "type": "object", "properties": {
            "twitter": {"type": "array", "items": {"type": "string"}},
            "instagram": {"type": "string"},
            "facebook": {"type": "string"}
        }}}
      prompt: |
        Write 3 Tweets, 1 IG caption, and 1 FB post about:
        {{marketing_context.summary}}

  - id: airtable_row
    use: airtable.records.create
    with:
      base_id: "{{secrets.AIR_BASE}}"
      table: "Launch Copy"
      fields:
        Feature: "{{event.feature}}"
        Twitter: '{{gen_copy.twitter | join("\n\n---\n\n")}}'
        Instagram: "{{gen_copy.instagram}}"
        Facebook: "{{gen_copy.facebook}}"
        Status: "Pending"

  - id: await_approval
    await_event:
      source: airtable
      match:
        record_id: "{{airtable_row.id}}"
        field: Status
        equals: Approved

  - id: parallel
    parallel: true

  - id: push_twitter
    foreach: "{{gen_copy.twitter}}"
    as: tweet
    do:
      - id: push_twitter_do
        use: twitter.tweet.create
        with:
          text: "{{tweet}}"

  - id: push_instagram
    use: instagram.media.create
    with:
      caption: "{{gen_copy.instagram}}"
      image_url: "{{event.image_url}}"

  - id: push_facebook
    use: facebook.post.create
    with:
      message: "{{gen_copy.facebook}}"
`

const minimalPushTwitterFlow = `name: minimal_push_twitter
on:
  - event: test
steps:
  - id: push_twitter
    foreach: "{{gen_copy.twitter}}"
    as: tweet
    do:
      - use: twitter.tweet.create
        with:
          text: "{{tweet}}"
`

const minimalPushTwitterAndInstagramFlow = `name: minimal_push_twitter_instagram
on:
  - event: test
steps:
  - id: push_twitter
    foreach: "{{gen_copy.twitter}}"
    as: tweet
    do:
      - use: twitter.tweet.create
        with:
          text: "{{tweet}}"
  - id: push_instagram
    use: instagram.media.create
    with:
      caption: "{{gen_copy.instagram}}"
      image_url: "{{event.image_url}}"
`

const minimalPushTwitterInstagramFacebookFlow = `name: minimal_push_twitter_instagram_facebook
on:
  - event: test
steps:
  - id: push_twitter
    foreach: "{{gen_copy.twitter}}"
    as: tweet
    do:
      - use: twitter.tweet.create
        with:
          text: "{{tweet}}"
  - id: push_instagram
    use: instagram.media.create
    with:
      caption: "{{gen_copy.instagram}}"
      image_url: "{{event.image_url}}"
  - id: push_facebook
    use: facebook.post.create
    with:
      message: "{{gen_copy.facebook}}"
`

const minimalPushTwitterInstagramFacebookParallelFlow = `name: minimal_push_twitter_instagram_facebook_parallel
on:
  - event: test
steps:
  - id: push_twitter
    foreach: "{{gen_copy.twitter}}"
    as: tweet
    do:
      - id: push_twitter_do
        use: twitter.tweet.create
        with:
          text: "{{tweet}}"
  - id: push_instagram
    use: instagram.media.create
    with:
      caption: "{{gen_copy.instagram}}"
      image_url: "{{event.image_url}}"
  - id: push_facebook
    use: facebook.post.create
    with:
      message: "{{gen_copy.facebook}}"
  - id: parallel
    parallel: true
`

// Negative and edge case YAMLs
const missingFieldsYAML = `
steps:
  - id: only_step
    use: core.echo
    with:
      text: 'hi'
`

const deepYAML = `
name: deep
on: cli.manual
steps:
  - id: outer
    parallel: true
  - id: inner1
    foreach: "{{list}}"
    as: item
    do:
      - id: inner1_do
        use: core.echo
        with:
          text: "{{item}}"
  - id: inner2
    use: core.echo
    with:
      text: "hi"
`

const unknownYAML = `
name: unknown
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with:
      text: hi
    unknown_field: 123
`

const emptyYAML = `
name: empty
on: cli.manual
steps: []
catch: {}
`

const allYAML = `
name: all_types
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with:
      text: hi
    if: "x > 0"
    foreach: "{{list}}"
    as: item
    do:
      - id: s1_do
        use: core.echo
        with:
          text: "{{item}}"
    parallel: true
    retry:
      attempts: 2
      delay_sec: 1
    await_event:
      source: bus
      match:
        key: value
      timeout: 10s
    wait:
      seconds: 5
      until: "2025-01-01"
  - id: s2
    use: core.echo
    with:
      text: hi
catch:
  e1:
    use: core.echo
    with:
      text: err
`

const badYAML = `
name: bad_steps
on: cli.manual
steps:
  - not_a_map
`

func TestParseFlow_HelloWorld(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "hello.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(helloWorldFlow)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}

	if flow.Name != "hello" {
		t.Errorf("expected flow name 'hello', got '%s'", flow.Name)
	}
	if flow.On == nil {
		t.Errorf("expected non-nil On trigger")
	}
	if len(flow.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(flow.Steps))
	}
	foundGreet, foundPrint := false, false
	for _, step := range flow.Steps {
		if step.ID == "greet" {
			foundGreet = true
		}
		if step.ID == "print" {
			foundPrint = true
		}
	}
	if !foundGreet {
		t.Errorf("expected step 'greet' in steps")
	}
	if !foundPrint {
		t.Errorf("expected step 'print' in steps")
	}
}

func TestParseFlow_MarketingBlast(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "marketing_blast.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(marketingBlastFlow)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}

	if flow.Name != "launch_blast" {
		t.Errorf("expected flow name 'launch_blast', got '%s'", flow.Name)
	}
	if flow.Vars == nil || flow.Vars["wait_between_polls"] != 30 {
		t.Errorf("expected vars.wait_between_polls == 30, got %#v", flow.Vars)
	}
	if len(flow.Steps) < 9 {
		t.Errorf("expected at least 9 steps, got %d", len(flow.Steps))
	}
	foundAwaitApproval, foundPushTwitter, foundParallel := false, false, false
	for _, step := range flow.Steps {
		if step.ID == "await_approval" {
			foundAwaitApproval = true
		}
		if step.ID == "push_twitter" {
			foundPushTwitter = true
		}
		if step.ID == "parallel" {
			foundParallel = true
		}
	}
	if !foundAwaitApproval {
		t.Errorf("expected step 'await_approval' in steps")
	}
	if !foundPushTwitter {
		t.Errorf("expected step 'push_twitter' in steps")
	}
	if !foundParallel {
		t.Errorf("expected step 'parallel' in steps")
	}
}

func TestParseFlow_MinimalPushTwitter(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "minimal_push_twitter.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(minimalPushTwitterFlow)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}

	if flow.Name != "minimal_push_twitter" {
		t.Errorf("expected flow name 'minimal_push_twitter', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(flow.Steps))
	}
	foundPushTwitter := false
	for _, step := range flow.Steps {
		if step.ID == "push_twitter" {
			foundPushTwitter = true
		}
	}
	if !foundPushTwitter {
		t.Errorf("expected step 'push_twitter' in steps")
	}
}

func TestParseFlow_MinimalPushTwitterAndInstagram(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "minimal_push_twitter_instagram.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(minimalPushTwitterAndInstagramFlow)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}

	if flow.Name != "minimal_push_twitter_instagram" {
		t.Errorf("expected flow name 'minimal_push_twitter_instagram', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(flow.Steps))
	}
	foundPushTwitter, foundPushInstagram := false, false
	for _, step := range flow.Steps {
		if step.ID == "push_twitter" {
			foundPushTwitter = true
		}
		if step.ID == "push_instagram" {
			foundPushInstagram = true
		}
	}
	if !foundPushTwitter {
		t.Errorf("expected step 'push_twitter' in steps")
	}
	if !foundPushInstagram {
		t.Errorf("expected step 'push_instagram' in steps")
	}
}

func TestParseFlow_MinimalPushTwitterInstagramFacebook(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "minimal_push_twitter_instagram_facebook.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(minimalPushTwitterInstagramFacebookFlow)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}

	if flow.Name != "minimal_push_twitter_instagram_facebook" {
		t.Errorf("expected flow name 'minimal_push_twitter_instagram_facebook', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(flow.Steps))
	}
	foundPushTwitter, foundPushInstagram, foundPushFacebook := false, false, false
	for _, step := range flow.Steps {
		if step.ID == "push_twitter" {
			foundPushTwitter = true
		}
		if step.ID == "push_instagram" {
			foundPushInstagram = true
		}
		if step.ID == "push_facebook" {
			foundPushFacebook = true
		}
	}
	if !foundPushTwitter {
		t.Errorf("expected step 'push_twitter' in steps")
	}
	if !foundPushInstagram {
		t.Errorf("expected step 'push_instagram' in steps")
	}
	if !foundPushFacebook {
		t.Errorf("expected step 'push_facebook' in steps")
	}
}

func TestParseFlow_MinimalPushTwitterInstagramFacebookParallel(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "minimal_push_twitter_instagram_facebook_parallel.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(minimalPushTwitterInstagramFacebookParallelFlow)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}

	if flow.Name != "minimal_push_twitter_instagram_facebook_parallel" {
		t.Errorf("expected flow name 'minimal_push_twitter_instagram_facebook_parallel', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(flow.Steps))
	}
	foundPushTwitter, foundPushInstagram, foundPushFacebook := false, false, false
	for _, step := range flow.Steps {
		if step.ID == "push_twitter" {
			foundPushTwitter = true
		}
		if step.ID == "push_instagram" {
			foundPushInstagram = true
		}
		if step.ID == "push_facebook" {
			foundPushFacebook = true
		}
	}
	if !foundPushTwitter {
		t.Errorf("expected step 'push_twitter' in steps")
	}
	if !foundPushInstagram {
		t.Errorf("expected step 'push_instagram' in steps")
	}
	if !foundPushFacebook {
		t.Errorf("expected step 'push_facebook' in steps")
	}
}

// --- Negative and Edge Case Tests ---

func TestParseFlow_InvalidYAML(t *testing.T) {
	invalidYAML := `name: bad_flow\nsteps: [this is: not valid yaml]`
	tmpfile, err := os.CreateTemp("", "invalid.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(invalidYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	_, err = ParseFlow(tmpfile.Name())
	if err == nil {
		t.Errorf("expected error for invalid YAML, got nil")
	}
}

func TestParseFlow_MissingRequiredFields(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "missing_fields.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(missingFieldsYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}
	if flow.Name != "" {
		t.Errorf("expected empty flow name, got '%s'", flow.Name)
	}
	if flow.On != nil {
		t.Errorf("expected nil On trigger, got %#v", flow.On)
	}
}

func TestParseFlow_MalformedStep(t *testing.T) {
	malformedStepYAML := `name: malformed\nsteps:\n  bad_step: not_a_map`
	tmpfile, err := os.CreateTemp("", "malformed_step.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(malformedStepYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	_, err = ParseFlow(tmpfile.Name())
	if err == nil {
		t.Errorf("expected error for malformed step, got nil")
	}
}

func TestParseFlow_DeeplyNestedSteps(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "deep.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(deepYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}
	if len(flow.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(flow.Steps))
	}
}

func TestParseFlow_UnknownFields(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "unknown.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(unknownYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	_, err = ParseFlow(tmpfile.Name())
	if err != nil {
		t.Errorf("ParseFlow failed with unknown field: %v", err)
	}
}

func TestParseFlow_EmptyStepsAndCatch(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "empty.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(emptyYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}
	if len(flow.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(flow.Steps))
	}
	if len(flow.Catch) != 0 {
		t.Errorf("expected 0 catch, got %d", len(flow.Catch))
	}
}

func TestParseFlow_AllStepTypes(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "all_types.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(allYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flow, err := ParseFlow(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFlow failed: %v", err)
	}
	if len(flow.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(flow.Steps))
	}
	if len(flow.Catch) != 1 {
		t.Errorf("expected 1 catch, got %d", len(flow.Catch))
	}
}

func TestParseFlow_InvalidStepsType(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "bad_steps.flow.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(badYAML)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	_, err = ParseFlow(tmpfile.Name())
	if err == nil {
		t.Errorf("expected error for steps as list, got nil")
	}
}

func TestParseFlow_FileNotExist(t *testing.T) {
	_, err := ParseFlow("/nonexistent/path/flow.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestValidateFlow_AllErrors(t *testing.T) {
	// 1. Marshal error: pass a struct with a channel (not serializable)
	type BadFlow struct {
		model.Flow
		C chan int
	}
	bad := &BadFlow{}
	// This will fail to marshal
	_ = bad // avoid unused warning
	// We need to call ValidateFlow with a *model.Flow, so we can't directly pass bad
	// Instead, simulate marshal error by passing a model.Flow with a field that can't be marshaled (not possible here), so skip this part

	// 2. Schema compile error: pass a non-existent schema file
	flow := &model.Flow{Name: "test"}
	err := ValidateFlow(flow, "/nonexistent/schema.json")
	if err == nil {
		t.Error("expected schema compile error, got nil")
	}

	// 3. JSON unmarshal error: pass a schema that expects a string but gets an object
	tmpSchema, err := os.CreateTemp("", "bad_schema.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmpSchema.Name())
	if _, err := tmpSchema.Write([]byte(`{"type":"string"}`)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmpSchema.Close()
	err = ValidateFlow(flow, tmpSchema.Name())
	if err == nil {
		t.Error("expected validation error, got nil")
	}
}

func TestValidateFlow_SchemaValidationError(t *testing.T) {
	// Create a schema that requires a field that is missing
	schema := `{"type":"object","required":["foo"],"properties":{"foo":{"type":"string"}}}`
	tmpSchema, err := os.CreateTemp("", "schema_validation_error.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmpSchema.Name())
	if _, err := tmpSchema.Write([]byte(schema)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmpSchema.Close()
	flow := &model.Flow{Name: "test"}
	err = ValidateFlow(flow, tmpSchema.Name())
	if err == nil {
		t.Error("expected schema validation error, got nil")
	}
}

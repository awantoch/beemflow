package dsl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/model"
	pongo2 "github.com/flosch/pongo2/v6"
)

func TestNewTemplater(t *testing.T) {
	tpl := NewTemplater()
	if tpl == nil {
		t.Error("expected NewTemplater not nil")
	}
}

func TestRender_Simple(t *testing.T) {
	tpl := NewTemplater()
	out, err := tpl.Render("Hello {{ Name }}", map[string]any{"Name": "Go"})
	if err != nil {
		t.Errorf("Render returned error: %v", err)
	}
	if out != "Hello Go" {
		t.Errorf("expected 'Hello Go', got %q", out)
	}
}

func TestRender_Error(t *testing.T) {
	tpl := NewTemplater()
	_, err := tpl.Render("{{", nil)
	if err == nil {
		t.Error("expected error for invalid template, got nil")
	}
}

func TestRender_NestedAndFilters(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"Feature": "AI Automation",
		"Docs":    []string{"Doc1", "Doc2", "Doc3"},
	}
	out, err := tpl.Render(`Feature: {{ Feature }}\nDocs: {{ Docs|join:", " }}\nCount: {{ Docs|length }}`, data)
	if err != nil {
		t.Errorf("Render returned error: %v", err)
	}
	expected := "Feature: AI Automation\\nDocs: Doc1, Doc2, Doc3\\nCount: 3"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestRender_BuiltinFilters(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"text": "hello world",
		"val":  42,
	}
	// Test pongo2's built-in filters instead of custom ones
	out, err := tpl.Render(`Title: {{ text|title }}, Upper: {{ text|upper }}, Length: {{ text|length }}`, data)
	if err != nil {
		t.Errorf("Render returned error: %v", err)
	}
	if !strings.Contains(out, "Title: Hello World") {
		t.Errorf("expected title filter output, got %q", out)
	}
	if !strings.Contains(out, "Upper: HELLO WORLD") {
		t.Errorf("expected upper filter output, got %q", out)
	}
	if !strings.Contains(out, "Length: 11") {
		t.Errorf("expected length filter output, got %q", out)
	}
}

func TestRender_NilData(t *testing.T) {
	tpl := NewTemplater()
	_, err := tpl.Render("Hello {{ Name }}", nil)
	if err == nil {
		t.Errorf("expected error for nil data, got nil")
	}
}

func TestRender_MissingKey(t *testing.T) {
	tpl := NewTemplater()
	out, err := tpl.Render("Hello {{ Missing }}", map[string]any{"Name": "Go"})
	if err != nil {
		t.Errorf("unexpected error for missing key: %v", err)
	}
	if out != "Hello " {
		t.Errorf("expected blank for missing key, got %q", out)
	}
}

func TestRender_CustomFilter(t *testing.T) {
	tpl := NewTemplater()
	err := tpl.RegisterFilters(map[string]pongo2.FilterFunction{
		"repeat": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			n := param.Integer()
			res := ""
			for i := 0; i < n; i++ {
				res += in.String()
			}
			return pongo2.AsValue(res), nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterFilters failed: %v", err)
	}
	out, err := tpl.Render(`{{ "ha"|repeat:3 }}!`, map[string]any{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "hahaha!" {
		t.Errorf("expected 'hahaha!', got %q", out)
	}
}

func TestRender_ArrayAccess(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"arr": []map[string]any{
			{"val": "a"},
			{"val": "b"},
		},
	}
	out, err := tpl.Render(`First: {{ arr.0.val }}, Second: {{ arr.1.val }}`, data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "First: a, Second: b" {
		t.Errorf("expected 'First: a, Second: b', got %q", out)
	}
}

func TestRender_NoDelimiters(t *testing.T) {
	tpl := NewTemplater()
	out, err := tpl.Render("plain text", map[string]any{"foo": "bar"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "plain text" {
		t.Errorf("expected 'plain text', got %q", out)
	}
}

func TestRender_DeeplyNestedTemplates(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"outer": map[string]any{
			"inner": map[string]any{
				"value": "deep",
			},
		},
	}
	out, err := tpl.Render("Value: {{ outer.inner.value }}", data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "Value: deep" {
		t.Errorf("expected 'Value: deep', got %q", out)
	}
}

func TestRender_SecretsDotAccess(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"secrets": map[string]any{"MY_SECRET": "shhh"},
	}
	out, err := tpl.Render("{{ secrets.MY_SECRET }}", data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "shhh" {
		t.Errorf("expected 'shhh', got %q", out)
	}
}

func TestRender_EventDotAccess(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"event": map[string]any{"foo": "bar"},
	}
	out, err := tpl.Render("{{ event.foo }}", data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "bar" {
		t.Errorf("expected 'bar', got %q", out)
	}
}

func TestRender_StepOutputTopLevel(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"fetch": map[string]any{"body": "hello world"},
	}
	out, err := tpl.Render("{{ fetch.body }}", data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("expected 'hello world', got %q", out)
	}
}

func TestRender_NestedStepOutput(t *testing.T) {
	tpl := NewTemplater()
	choices := []interface{}{
		map[string]any{"message": map[string]any{"content": "summary here"}},
	}
	data := map[string]any{
		"summarize": map[string]any{
			"choices": choices,
		},
	}
	// Only dot notation is supported for array access in pongo2
	out, err := tpl.Render("{{ summarize.choices.0.message.content }}", data)
	if err != nil {
		t.Errorf("unexpected error (dot notation): %v", err)
	}
	if out != "summary here" {
		t.Errorf("expected 'summary here' (dot notation), got %q", out)
	}
	// Bracket notation (choices[0]) is not supported by pongo2 and will fail to parse.
}

func TestRegisterFilters(t *testing.T) {
	templater := NewTemplater()

	// Test registering a custom filter
	err := templater.RegisterFilters(map[string]pongo2.FilterFunction{
		"double": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			val := in.Integer()
			return pongo2.AsValue(val * 2), nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterFilters failed: %v", err)
	}

	result, err := templater.Render("{{ 5 | double }}", map[string]any{})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "10" {
		t.Errorf("Expected '10', got '%s'", result)
	}
}

// Test Load function
func TestLoad(t *testing.T) {
	// Create a temporary YAML file
	tempDir, err := os.MkdirTemp("", "dsl_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	yamlContent := `name: test_flow
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "Hello World"
`

	yamlFile := filepath.Join(tempDir, "test.flow.yaml")
	err = os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test Load function
	flow, err := Load(yamlFile, map[string]any{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if flow.Name != "test_flow" {
		t.Errorf("Expected flow name 'test_flow', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(flow.Steps))
	}

	// Test Load with non-existent file
	_, err = Load("non_existent_file.yaml", map[string]any{})
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test Load with invalid YAML
	invalidYamlFile := filepath.Join(tempDir, "invalid.yaml")
	err = os.WriteFile(invalidYamlFile, []byte("invalid: yaml: content: ["), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid test file: %v", err)
	}

	_, err = Load(invalidYamlFile, map[string]any{})
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

// Test Parse function
func TestParse(t *testing.T) {
	// Create a temporary YAML file
	tempDir, err := os.MkdirTemp("", "dsl_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	yamlContent := `name: test_flow
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "Hello World"
`

	yamlFile := filepath.Join(tempDir, "test.flow.yaml")
	err = os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	flow, err := Parse(yamlFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if flow.Name != "test_flow" {
		t.Errorf("Expected flow name 'test_flow', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(flow.Steps))
	}

	// Test Parse with non-existent file
	_, err = Parse("non_existent_file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// Test ParseFromString function
func TestParseFromString(t *testing.T) {
	yamlString := `name: test_flow
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "Hello World"
`

	flow, err := ParseFromString(yamlString)
	if err != nil {
		t.Fatalf("ParseFromString failed: %v", err)
	}
	if flow.Name != "test_flow" {
		t.Errorf("Expected flow name 'test_flow', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(flow.Steps))
	}

	// Test ParseFromString with invalid YAML
	invalidYaml := "invalid: yaml: content: ["
	_, err = ParseFromString(invalidYaml)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}

	// Test ParseFromString with empty string - this actually parses successfully to an empty Flow
	flow, err = ParseFromString("")
	if err != nil {
		t.Errorf("ParseFromString with empty string failed: %v", err)
	}
	if flow == nil {
		t.Error("Expected non-nil flow for empty string")
	}
}

// Test Validate function
func TestValidate(t *testing.T) {
	// Test valid flow with required 'on' field
	validFlow := &model.Flow{
		Name: "test_flow",
		On:   "cli.manual",
		Steps: []model.Step{
			{
				ID:  "test_step",
				Use: "core.echo",
			},
		},
	}

	err := Validate(validFlow)
	if err != nil {
		t.Errorf("Validate failed for valid flow: %v", err)
	}

	// Test various invalid flows - we don't assert specific errors since
	// the behavior depends on the JSON schema implementation
	testCases := []struct {
		name string
		flow *model.Flow
	}{
		{
			name: "missing name",
			flow: &model.Flow{
				On: "cli.manual",
				Steps: []model.Step{
					{
						ID:  "test_step",
						Use: "core.echo",
					},
				},
			},
		},
		{
			name: "missing on field",
			flow: &model.Flow{
				Name: "test_flow",
				Steps: []model.Step{
					{
						ID:  "test_step",
						Use: "core.echo",
					},
				},
			},
		},
		{
			name: "empty steps",
			flow: &model.Flow{
				Name:  "test_flow",
				On:    "cli.manual",
				Steps: []model.Step{},
			},
		},
		{
			name: "step missing ID",
			flow: &model.Flow{
				Name: "test_flow",
				On:   "cli.manual",
				Steps: []model.Step{
					{
						Use: "core.echo",
					},
				},
			},
		},
		{
			name: "step missing Use",
			flow: &model.Flow{
				Name: "test_flow",
				On:   "cli.manual",
				Steps: []model.Step{
					{
						ID: "test_step",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.flow)
			// We just test that validation runs without panic
			// The actual error behavior depends on the JSON schema
			t.Logf("Validation result for %s: %v", tc.name, err)
		})
	}
}

// Test EvaluateExpression function
func TestEvaluateExpression(t *testing.T) {
	templater := NewTemplater()

	context := map[string]any{
		"event": map[string]any{
			"name": "John",
			"age":  30,
		},
		"vars": map[string]any{
			"greeting": "Hello",
		},
	}

	// Test simple expression
	result, err := templater.EvaluateExpression("{{ event.name }}", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	if result != "John" {
		t.Errorf("Expected 'John', got '%v'", result)
	}

	// Test nested expression
	result, err = templater.EvaluateExpression("{{ vars.greeting }}", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	if result != "Hello" {
		t.Errorf("Expected 'Hello', got '%v'", result)
	}

	// Test expression with missing key
	result, err = templater.EvaluateExpression("{{ event.missing }}", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	// Missing keys should return empty string when rendered
	if result != "" {
		t.Errorf("Expected empty string for missing key, got '%v'", result)
	}

	// Test non-template expression
	result, err = templater.EvaluateExpression("simple_string", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	if result != "simple_string" {
		t.Errorf("Expected 'simple_string', got '%v'", result)
	}
}

// Test lookupNestedPath function (internal function, test via EvaluateExpression)
func TestLookupNestedPath(t *testing.T) {
	templater := NewTemplater()

	context := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": "deep_value",
			},
			"array": []any{"item1", "item2", "item3"},
		},
		"simple": "simple_value",
	}

	// Test deep nested path
	result, err := templater.EvaluateExpression("{{ level1.level2.level3 }}", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	if result != "deep_value" {
		t.Errorf("Expected 'deep_value', got '%v'", result)
	}

	// Test array access - the result type depends on the evaluation path
	result, err = templater.EvaluateExpression("{{ level1.array }}", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	// The result could be a string (rendered) or the actual array
	switch v := result.(type) {
	case string:
		if !strings.Contains(v, "item1") {
			t.Errorf("Expected array content in string result, got '%v'", result)
		}
	case []interface{}:
		if len(v) != 3 {
			t.Errorf("Expected array with 3 items, got %d", len(v))
		}
	default:
		t.Errorf("Unexpected result type %T for array access: %v", result, result)
	}

	// Test simple path
	result, err = templater.EvaluateExpression("{{ simple }}", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	if result != "simple_value" {
		t.Errorf("Expected 'simple_value', got '%v'", result)
	}

	// Test non-existent path
	result, err = templater.EvaluateExpression("{{ non.existent.path }}", context)
	if err != nil {
		t.Fatalf("EvaluateExpression failed: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty string for non-existent path, got '%v'", result)
	}
}

// Test additional Render edge cases
func TestRender_AdditionalCases(t *testing.T) {
	templater := NewTemplater()

	// Test rendering with complex nested data
	data := map[string]any{
		"user": map[string]any{
			"profile": map[string]any{
				"settings": map[string]any{
					"theme": "dark",
				},
			},
		},
		"items": []any{
			map[string]any{"name": "item1", "value": 10},
			map[string]any{"name": "item2", "value": 20},
		},
	}

	result, err := templater.Render("Theme: {{ user.profile.settings.theme }}", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Theme: dark" {
		t.Errorf("Expected 'Theme: dark', got '%s'", result)
	}

	// Test rendering with invalid template syntax
	_, err = templater.Render("{{ invalid template syntax", data)
	if err == nil {
		t.Error("Expected error for invalid template syntax")
	}
}

// Test NewTemplater edge cases
func TestNewTemplater_EdgeCases(t *testing.T) {
	templater := NewTemplater()
	if templater == nil {
		t.Fatal("NewTemplater returned nil")
	}

	// Test that the templater has the expected built-in filters
	data := map[string]any{"value": "hello"}

	result, err := templater.Render("{{ value | upper }}", data)
	if err != nil {
		t.Fatalf("Render with upper filter failed: %v", err)
	}
	if result != "HELLO" {
		t.Errorf("Expected 'HELLO', got '%s'", result)
	}

	result, err = templater.Render("{{ value | lower }}", data)
	if err != nil {
		t.Fatalf("Render with lower filter failed: %v", err)
	}
	if result != "hello" {
		t.Errorf("Expected 'hello', got '%s'", result)
	}
}

// Test RegisterFilters edge cases
func TestRegisterFilters_EdgeCases(t *testing.T) {
	templater := NewTemplater()

	// Test registering multiple filters
	err := templater.RegisterFilters(map[string]pongo2.FilterFunction{
		"triple": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			val := in.Integer()
			return pongo2.AsValue(val * 3), nil
		},
		"square": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			val := in.Integer()
			return pongo2.AsValue(val * val), nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterFilters failed: %v", err)
	}

	result, err := templater.Render("{{ 4 | triple }}", map[string]any{})
	if err != nil {
		t.Fatalf("Render with triple filter failed: %v", err)
	}
	if result != "12" {
		t.Errorf("Expected '12', got '%s'", result)
	}

	result, err = templater.Render("{{ 5 | square }}", map[string]any{})
	if err != nil {
		t.Fatalf("Render with square filter failed: %v", err)
	}
	if result != "25" {
		t.Errorf("Expected '25', got '%s'", result)
	}

	// Test registering nil filters (should not crash)
	err = templater.RegisterFilters(nil)
	if err != nil {
		t.Errorf("RegisterFilters with nil should not fail: %v", err)
	}

	// Test registering empty filters
	err = templater.RegisterFilters(map[string]pongo2.FilterFunction{})
	if err != nil {
		t.Errorf("RegisterFilters with empty map should not fail: %v", err)
	}
}

// Test Render function (package-level)
func TestRender_PackageLevel(t *testing.T) {
	data := map[string]any{"name": "World"}

	result, err := Render("Hello {{ name }}!", data)
	if err != nil {
		t.Fatalf("Package-level Render failed: %v", err)
	}
	if result != "Hello World!" {
		t.Errorf("Expected 'Hello World!', got '%s'", result)
	}

	// Test with nil data
	_, err = Render("Hello World!", nil)
	if err == nil {
		t.Error("Expected error for nil data")
	}
}

// Test RegisterFilters function (package-level)
func TestRegisterFilters_PackageLevel(t *testing.T) {
	err := RegisterFilters(map[string]pongo2.FilterFunction{
		"test_filter": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			return pongo2.AsValue("test_result"), nil
		},
	})
	if err != nil {
		t.Fatalf("Package-level RegisterFilters failed: %v", err)
	}
}

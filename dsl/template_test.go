package dsl

import (
	"strings"
	"testing"

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
		"val": 42,
	}
	out, err := tpl.Render(`Base64: {{ "hi"|base64 }}, Now: {{ ""|now }}, Duration: {{ val|duration:"h" }}`, data)
	if err != nil {
		t.Errorf("Render returned error: %v", err)
	}
	if !strings.Contains(out, "Base64: aGk=") {
		t.Errorf("expected base64 output, got %q", out)
	}
	if !strings.Contains(out, "Duration: 42h") {
		t.Errorf("expected duration output, got %q", out)
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
	tpl.RegisterFilters(map[string]pongo2.FilterFunction{
		"repeat": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			n := param.Integer()
			res := ""
			for i := 0; i < n; i++ {
				res += in.String()
			}
			return pongo2.AsValue(res), nil
		},
	})
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

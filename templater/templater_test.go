package templater

import (
	"encoding/base64"
	"strings"
	"testing"
	"text/template"
)

func helpers() template.FuncMap {
	return template.FuncMap{
		"join": func(arr []string, sep string) string {
			return strings.Join(arr, sep)
		},
		"length": func(arr any) int {
			switch v := arr.(type) {
			case []string:
				return len(v)
			case []any:
				return len(v)
			default:
				return 0
			}
		},
		"base64": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"toUpper": func(s string) string {
			return strings.ToUpper(s)
		},
		"map": func(arr []string, fn string) []string {
			if fn == "toUpper" {
				res := make([]string, len(arr))
				for i, v := range arr {
					res[i] = strings.ToUpper(v)
				}
				return res
			}
			return arr
		},
	}
}

func TestNewTemplater(t *testing.T) {
	tpl := NewTemplater()
	if tpl == nil {
		t.Error("expected NewTemplater not nil")
	}
}

func TestRender_Simple(t *testing.T) {
	tpl := NewTemplater()
	out, err := tpl.Render("Hello {{.Name}}", map[string]any{"Name": "Go"})
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

func TestRegisterHelpers_NoPanic(t *testing.T) {
	tpl := NewTemplater()
	// Should not panic
	tpl.RegisterHelpers(nil)
}

func TestRender_NestedAndHelpers(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"Feature": "AI Automation",
		"Docs":    []string{"Doc1", "Doc2", "Doc3"},
	}
	tpl.RegisterHelpers(helpers())
	out, err := tpl.Render(`Feature: {{.Feature}}\nDocs: {{join .Docs ", "}}\nCount: {{length .Docs}}`, data)
	if err != nil {
		t.Errorf("Render returned error: %v", err)
	}
	expected := "Feature: AI Automation\\nDocs: Doc1, Doc2, Doc3\\nCount: 3"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestRender_AllHelpers(t *testing.T) {
	tpl := NewTemplater()
	data := map[string]any{
		"arr": []string{"a", "b", "c"},
		"val": 42,
	}
	tpl.RegisterHelpers(helpers())
	out, err := tpl.Render(`Len: {{length .arr}}, Join: {{join .arr ","}}, Base64: {{base64 "hi"}}`, data)
	if err != nil {
		t.Errorf("Render returned error: %v", err)
	}
	if out == "" || out == "Len: , Join: , Base64: " {
		t.Errorf("expected helpers output, got %q", out)
	}
}

func TestRender_NilData(t *testing.T) {
	tpl := NewTemplater()
	_, err := tpl.Render("Hello {{.Name}}", nil)
	if err == nil {
		t.Errorf("expected error for nil data, got nil")
	}
}

func TestRender_MissingKey(t *testing.T) {
	tpl := NewTemplater()
	out, err := tpl.Render("Hello {{.Missing}}", map[string]any{"Name": "Go"})
	if err == nil {
		t.Errorf("expected error for missing key, got nil")
	}
	if out != "" {
		t.Errorf("expected empty output for missing key, got %q", out)
	}
}

func TestRender_ErrorPropagation(t *testing.T) {
	tpl := NewTemplater()
	_, err := tpl.Render("{{fail}}", map[string]any{})
	if err == nil {
		t.Errorf("expected error for unknown helper, got nil")
	}
}

func TestRender_NestedHelpers(t *testing.T) {
	tpl := NewTemplater()
	tpl.RegisterHelpers(helpers())
	data := map[string]any{"arr": []string{"a", "b"}}
	out, err := tpl.Render(`{{join (map .arr "toUpper") ","}}`, data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "A,B" {
		t.Errorf("expected 'A,B', got %q", out)
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
	tpl.RegisterHelpers(helpers())
	data := map[string]any{
		"outer": map[string]any{
			"inner": map[string]any{
				"value": "deep",
			},
		},
	}
	out, err := tpl.Render("Value: {{.outer.inner.value}}", data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "Value: deep" {
		t.Errorf("expected 'Value: deep', got %q", out)
	}
}

func TestRender_CustomHelperComplex(t *testing.T) {
	tpl := NewTemplater()
	tpl.RegisterHelpers(template.FuncMap{
		"repeat": func(s string, n int) string {
			res := ""
			for i := 0; i < n; i++ {
				res += s
			}
			return res
		},
	})
	out, err := tpl.Render("{{repeat \"ha\" 3}}!", map[string]any{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != "hahaha!" {
		t.Errorf("expected 'hahaha!', got %q", out)
	}
}

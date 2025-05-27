package dsl

import (
	"fmt"
	"maps"
	"strings"

	"github.com/awantoch/beemflow/utils"
	pongo2 "github.com/flosch/pongo2/v6"
)

// Templater provides template rendering with Jinja2-style (pongo2) for BeemFlow flows.
type Templater struct{}

// NewTemplater creates a new Templater.
func NewTemplater() *Templater {
	return &Templater{}
}

// Render renders a template string with the provided data using pongo2.
func (t *Templater) Render(tmpl string, data map[string]any) (string, error) {
	if data == nil {
		return "", fmt.Errorf("template data is nil")
	}
	ctx := flattenContext(data)
	// DEBUG: Log template string and context keys before rendering
	utils.Debug("Templater.Render: tmpl = %q, context keys = %v", tmpl, contextKeys(ctx))
	pl, err := pongo2.FromString(tmpl)
	if err != nil {
		return "", err
	}
	out, err := pl.Execute(ctx)
	if err != nil {
		return "", err
	}
	return out, nil
}

// RegisterFilters allows registering custom pongo2 filters.
func (t *Templater) RegisterFilters(filters map[string]pongo2.FilterFunction) {
	for name, fn := range filters {
		_ = pongo2.RegisterFilter(name, fn)
	}
}

// Render applies templating to the given string with the provided data.
func Render(tmpl string, data map[string]any) (string, error) {
	return NewTemplater().Render(tmpl, data)
}

// RegisterFilters applies custom filters for rendering.
func RegisterFilters(filters map[string]pongo2.FilterFunction) {
	NewTemplater().RegisterFilters(filters)
}

// flattenContext converts the map for pongo2 compatibility.
func flattenContext(data map[string]any) pongo2.Context {
	converted := make(pongo2.Context, len(data))
	maps.Copy(converted, data)
	return converted
}

// contextKeys returns the keys of a pongo2.Context as a []string.
func contextKeys(ctx pongo2.Context) []string {
	var out []string
	for k := range ctx {
		out = append(out, k)
	}
	return out
}

// EvaluateExpression evaluates a template expression and returns the actual value
// instead of rendering it to a string. This is useful for foreach expressions
// that need to extract lists or other non-string values.
func (t *Templater) EvaluateExpression(tmpl string, data map[string]any) (any, error) {
	if data == nil {
		return nil, fmt.Errorf("template data is nil")
	}

	// If it's not a template expression, return as-is
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}

	// Check if it's a simple variable expression like {{ vars.analysis_types }} or {{ list }}
	// We need to extract the variable path and look it up directly in the context
	tmplTrimmed := strings.TrimSpace(tmpl)
	if strings.HasPrefix(tmplTrimmed, "{{") && strings.HasSuffix(tmplTrimmed, "}}") {
		// Extract the variable path from {{ vars.analysis_types }}
		varPath := strings.TrimSpace(tmplTrimmed[2 : len(tmplTrimmed)-2])

		// Create flattened context for lookup
		ctx := flattenContext(data)

		// Simple case: direct variable lookup like "vars.analysis_types"
		if strings.Contains(varPath, ".") {
			parts := strings.Split(varPath, ".")
			if len(parts) == 2 {
				if contextMap, ok := data[parts[0]].(map[string]any); ok {
					if value, exists := contextMap[parts[1]]; exists {
						return value, nil
					}
				}
			}
		}

		// Try flattened context lookup (vars are flattened into top-level)
		if val, exists := ctx[varPath]; exists {
			return val, nil
		}

		// Try direct lookup in top-level data (for event variables like "list")
		if val, exists := data[varPath]; exists {
			return val, nil
		}
	}

	// Fallback to string rendering for complex expressions
	return t.Render(tmpl, data)
}

// Helper function to get map keys for debugging
func keys(m map[string]any) []string {
	var result []string
	for k := range m {
		result = append(result, k)
	}
	return result
}

package dsl

import (
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/awantoch/beemflow/utils"
	pongo2 "github.com/flosch/pongo2/v6"
)

var (
	// Global filter registration to avoid duplicate registrations
	filterRegistrationOnce sync.Once
)

// Templater provides template rendering with Jinja2-style (pongo2) for BeemFlow flows.
type Templater struct {
	mu sync.Mutex // protects pongo2 operations which are not thread-safe
}

// NewTemplater creates a new Templater.
func NewTemplater() *Templater {
	t := &Templater{}

	// Register default custom filters only once globally
	filterRegistrationOnce.Do(func() {
		defaultFilters := map[string]pongo2.FilterFunction{
			"reverse": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
				s := in.String()
				runes := []rune(s)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return pongo2.AsValue(string(runes)), nil
			},
		}

		// Register the filters globally
		for name, fn := range defaultFilters {
			if err := pongo2.RegisterFilter(name, fn); err != nil {
				// Log the error but don't fail - the filter might already exist
				utils.Debug("Filter registration warning: %v", err)
			}
		}
	})

	return t
}

// Render renders a template string with the provided data using pongo2.
func (t *Templater) Render(tmpl string, data map[string]any) (string, error) {
	if data == nil {
		return "", fmt.Errorf("template data is nil")
	}
	ctx := flattenContext(data)
	// DEBUG: Log template string and context keys before rendering
	utils.Debug("Templater.Render: tmpl = %q, context keys = %v", tmpl, contextKeys(ctx))

	// Protect pongo2 operations with mutex since they're not thread-safe
	t.mu.Lock()
	pl, err := pongo2.FromString(tmpl)
	if err != nil {
		t.mu.Unlock()
		return "", err
	}
	out, err := pl.Execute(ctx)
	t.mu.Unlock()

	if err != nil {
		return "", err
	}
	return out, nil
}

// RegisterFilters allows registering custom pongo2 filters.
func (t *Templater) RegisterFilters(filters map[string]pongo2.FilterFunction) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name, fn := range filters {
		if err := pongo2.RegisterFilter(name, fn); err != nil {
			return utils.Errorf("failed to register filter %s: %w", name, err)
		}
	}
	return nil
}

// Render applies templating to the given string with the provided data.
func Render(tmpl string, data map[string]any) (string, error) {
	return NewTemplater().Render(tmpl, data)
}

// RegisterFilters applies custom filters for rendering.
func RegisterFilters(filters map[string]pongo2.FilterFunction) error {
	return NewTemplater().RegisterFilters(filters)
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

		// Try nested path lookup (e.g., "nested_data.level1.level2.array")
		if strings.Contains(varPath, ".") {
			if value, found := lookupNestedPath(varPath, data); found {
				return value, nil
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

// lookupNestedPath traverses a nested path like "nested_data.level1.level2.array" in the data map
func lookupNestedPath(path string, data map[string]any) (any, bool) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if val, exists := current[part]; exists {
			if i == len(parts)-1 {
				// Last part, return the value
				return val, true
			}
			// Not the last part, continue traversing
			if nextMap, ok := val.(map[string]any); ok {
				current = nextMap
			} else {
				// Can't traverse further
				return nil, false
			}
		} else {
			// Part not found
			return nil, false
		}
	}

	return nil, false
}

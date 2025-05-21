package templater

import (
	"fmt"
	"time"

	"encoding/base64"

	"github.com/awantoch/beemflow/logger"
	pongo2 "github.com/flosch/pongo2/v6"
)

// Templater provides template rendering with Jinja2-style (pongo2) for BeemFlow flows.
type Templater struct{}

// NewTemplater creates a new Templater and registers built-in filters.
func NewTemplater() *Templater {
	// Register built-in filters only once
	registerBuiltinFilters()
	return &Templater{}
}

// Render renders a template string with the provided data using pongo2.
func (t *Templater) Render(tmpl string, data map[string]any) (string, error) {
	if data == nil {
		return "", fmt.Errorf("template data is nil")
	}
	ctx := flattenContext(data)
	// DEBUG: Log template string and context keys before rendering
	logger.Debug("Templater.Render: tmpl = %q, context keys = %v", tmpl, contextKeys(ctx))
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

// registerBuiltinFilters registers built-in filters for BeemFlow.
var builtinFiltersRegistered = false

func registerBuiltinFilters() {
	if builtinFiltersRegistered {
		return
	}
	_ = pongo2.RegisterFilter("base64", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		return pongo2.AsValue(base64.StdEncoding.EncodeToString([]byte(in.String()))), nil
	})
	_ = pongo2.RegisterFilter("now", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		return pongo2.AsValue(time.Now().Format(time.RFC3339)), nil
	})
	_ = pongo2.RegisterFilter("duration", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		unit := param.String()
		var n int64
		switch v := in.Interface().(type) {
		case int:
			n = int64(v)
		case int64:
			n = v
		case float64:
			n = int64(v)
		case string:
			fmt.Sscanf(v, "%d", &n)
		}
		suffix := unit
		switch unit {
		case "days", "day", "d":
			suffix = "d"
		case "hours", "hour", "h":
			suffix = "h"
		case "minutes", "minute", "min", "m":
			suffix = "m"
		case "seconds", "second", "sec", "s":
			suffix = "s"
		}
		return pongo2.AsValue(fmt.Sprintf("%d%s", n, suffix)), nil
	})
	builtinFiltersRegistered = true
}

// flattenContext now just converts the map for pongo2 compatibility, since vars are already flattened in the engine.
func flattenContext(data map[string]any) pongo2.Context {
	converted := make(pongo2.Context)
	for k, v := range data {
		converted[k] = v
	}
	return converted
}

// contextKeys returns the keys of a pongo2.Context as a []string
func contextKeys(ctx pongo2.Context) []string {
	var out []string
	for k := range ctx {
		out = append(out, k)
	}
	return out
}

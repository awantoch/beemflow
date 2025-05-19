package templater

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/awantoch/beemflow/logger"
)

// Templater provides template rendering with helper functions for BeemFlow flows.
type Templater struct {
	helperFuncs template.FuncMap
	// You can register custom helpers using RegisterHelpers. Example:
	//   t.RegisterHelpers(template.FuncMap{"myfunc": func(x string) string { return ... }})
	// For more helpers, consider integrating github.com/Masterminds/sprig in the future.
}

// NewTemplater creates a new Templater with built-in helper functions.
func NewTemplater() *Templater {
	t := &Templater{
		helperFuncs: make(template.FuncMap),
	}
	t.RegisterHelpers(template.FuncMap{
		"eq": func(a any, b any) bool {
			return reflect.DeepEqual(a, b)
		},
		"ne": func(a any, b any) bool {
			return !reflect.DeepEqual(a, b)
		},
		"list": func(args ...any) []any { return args },
		"join": func(arr []any, sep string) string {
			var s []string
			for _, v := range arr {
				s = append(s, toString(v))
			}
			return strings.Join(s, sep)
		},
		"length": func(arr []any) int { return len(arr) },
		"base64": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"map": func(arr []any, field string) []any {
			var out []any
			for _, v := range arr {
				m, ok := v.(map[string]any)
				if !ok {
					continue
				}
				if f, ok := m[field]; ok {
					out = append(out, f)
				}
			}
			return out
		},
		"now": func() string {
			return time.Now().Format(time.RFC3339)
		},
		"duration": func(v any, unit string) string {
			var n int64
			switch x := v.(type) {
			case int:
				n = int64(x)
			case int64:
				n = x
			case float64:
				n = int64(x)
			default:
				return ""
			}
			var suffix string
			switch unit {
			case "days", "day", "d":
				suffix = "d"
			case "hours", "hour", "h":
				suffix = "h"
			case "minutes", "minute", "min", "m":
				suffix = "m"
			case "seconds", "second", "sec", "s":
				suffix = "s"
			default:
				suffix = unit
			}
			return fmt.Sprintf("%d%s", n, suffix)
		},
	})
	return t
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", x)
	}
}

func (t *Templater) Render(tmpl string, data map[string]any) (string, error) {
	const maxIterations = 5
	prev := tmpl
	for i := 0; i < maxIterations; i++ {
		tpl := template.New("").Option("missingkey=error")
		if t.helperFuncs != nil {
			tpl = tpl.Funcs(t.helperFuncs)
		}
		var err error
		tpl, err = tpl.Parse(prev)
		if err != nil {
			if i > 0 {
				// Runtime data may contain literal template delimiters; stop further parsing
				return prev, nil
			}
			return "", err
		}
		if data == nil {
			return "", logger.Errorf("template data is nil")
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, data); err != nil {
			return "", err
		}
		result := buf.String()
		if result == prev {
			return result, nil // stabilized
		}
		prev = result
	}
	return prev, nil // return after max iterations
}

func (t *Templater) RegisterHelpers(funcs interface{}) {
	if funcs == nil {
		return
	}
	switch m := funcs.(type) {
	case template.FuncMap:
		for k, v := range m {
			t.helperFuncs[k] = v
		}
	case map[string]any:
		for k, v := range m {
			t.helperFuncs[k] = v
		}
	}
}

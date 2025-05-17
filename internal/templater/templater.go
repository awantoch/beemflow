package templater

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type Templater struct {
	helperFuncs template.FuncMap
	// TODO: add custom funcs, sprig, etc.
}

func NewTemplater() *Templater {
	t := &Templater{
		helperFuncs: make(template.FuncMap),
	}
	t.RegisterHelpers(template.FuncMap{
		"list": func(args ...any) []any { return args },
		"join": func(arr []any, sep string) string {
			var s []string
			for _, v := range arr {
				s = append(s, toString(v))
			}
			return strings.Join(s, sep)
		},
		"length": func(arr []any) int { return len(arr) },
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
	tpl := template.New("").Option("missingkey=error")
	if t.helperFuncs != nil {
		tpl = tpl.Funcs(t.helperFuncs)
	}
	tpl, err := tpl.Parse(tmpl)
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", template.ExecError{Err: err}
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (t *Templater) RegisterHelpers(funcs template.FuncMap) {
	if funcs == nil {
		return
	}
	for k, v := range funcs {
		t.helperFuncs[k] = v
	}
}

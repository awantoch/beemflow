package convert

import (
	"bytes"
	"encoding/json"
	"io"

	jsonnet "github.com/google/go-jsonnet"
	"gopkg.in/yaml.v3"
)

// YAMLToJsonnet converts a YAML document (provided as bytes or string) to
// pretty-printed Jsonnet (which, for now, is valid JSON). The resulting string
// can be saved as a .jsonnet file and evaluated by any Jsonnet VM.
func YAMLToJsonnet(in []byte) (string, error) {
	var data any
	if err := yaml.Unmarshal(in, &data); err != nil {
		return "", err
	}
	// Marshal as indented JSON (valid Jsonnet by definition).
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

// JsonnetToYAML evaluates a Jsonnet snippet/file (passed as string/bytes) and
// returns a YAML representation using the same indentation rules as the rest
// of the codebase.
func JsonnetToYAML(in []byte) (string, error) {
	vm := jsonnet.MakeVM()
	jsonStr, err := vm.EvaluateSnippet("snippet", string(in))
	if err != nil {
		return "", err
	}
	// jsonStr is raw JSON – convert to YAML
	var buf bytes.Buffer
	if err := jsonToYAML(&buf, []byte(jsonStr)); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// jsonToYAML converts JSON bytes to YAML and writes to the provided writer.
func jsonToYAML(w io.Writer, jsonData []byte) error {
	var data any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return err
	}
	return enc.Close()
}
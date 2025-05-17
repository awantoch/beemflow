package main

import (
	"encoding/json"
	"os"
)

// loadEvent loads event data from a file or an inline JSON string.
func loadEvent(path, inline string) (map[string]any, error) {
	var event map[string]any
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		return event, nil
	}
	if inline != "" {
		if err := json.Unmarshal([]byte(inline), &event); err != nil {
			return nil, err
		}
		return event, nil
	}
	// No event provided: return empty event for flows that don't use event data
	return map[string]any{}, nil
}

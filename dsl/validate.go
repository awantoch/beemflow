package dsl

import (
	"encoding/json"

	"github.com/awantoch/beemflow/model"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Validate runs JSON-Schema validation against the embedded BeemFlow schema.
func Validate(flow *model.Flow) error {
	// Marshal the flow to JSON for validation
	jsonBytes, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	// Compile the embedded schema
	schema, err := jsonschema.CompileString("beemflow.schema.json", string(schemaJSON))
	if err != nil {
		return err
	}
	// Unmarshal JSON into a generic interface for validation
	var doc interface{}
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return err
	}
	// Validate the flow
	return schema.Validate(doc)
}

package dsl

import (
	"encoding/json"
	"fmt"

	"github.com/awantoch/beemflow/docs"
	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

// Validate runs JSON-Schema validation against the embedded BeemFlow schema for pproto.Flow.
func Validate(flow *pproto.Flow) error {
	// Marshal the flow to JSON for validation
	jsonBytes, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	// Compile the embedded schema
	schema, err := jsonschema.CompileString("beemflow.schema.json", docs.BeemflowSchema)
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

// ValidateProto runs JSON-Schema validation against the embedded BeemFlow schema for proto.Flow.
func ValidateProto(pb *pproto.Flow) error {
	// Marshal proto.Flow to JSON
	jsonBytes, err := (&protojson.MarshalOptions{EmitUnpopulated: true}).Marshal(pb)
	if err != nil {
		return fmt.Errorf("failed to marshal proto.Flow to JSON: %w", err)
	}
	// Compile the embedded schema
	schema, err := jsonschema.CompileString("beemflow.schema.json", docs.BeemflowSchema)
	if err != nil {
		return err
	}
	// Unmarshal JSON into generic interface for validation
	var doc interface{}
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return fmt.Errorf("failed to unmarshal JSON bytes: %w", err)
	}
	// Validate the flow
	return schema.Validate(doc)
}

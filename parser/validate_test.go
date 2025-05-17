package parser

import (
	"os"
	"testing"

	"github.com/awantoch/beemflow/model"
)

func TestValidateFlow_Success(t *testing.T) {
	schema := `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"properties": { "name": {"type": "string", "minLength": 1} },
	"required": ["name"]
}`
	tmp, err := os.CreateTemp("", "schema-*.json")
	if err != nil {
		t.Fatalf("failed to create temp schema: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write([]byte(schema)); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}
	tmp.Close()

	flow := &model.Flow{Name: "abc"}
	if err := ValidateFlow(flow, tmp.Name()); err != nil {
		t.Errorf("expected no validation error, got %v", err)
	}
}

func TestValidateFlow_Failure(t *testing.T) {
	schema := `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"properties": { "name": {"type": "string", "minLength": 1} },
	"required": ["name"]
}`
	tmp, err := os.CreateTemp("", "schema-*.json")
	if err != nil {
		t.Fatalf("failed to create temp schema: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write([]byte(schema)); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}
	tmp.Close()

	flow := &model.Flow{}
	if err := ValidateFlow(flow, tmp.Name()); err == nil {
		t.Errorf("expected validation error, got nil")
	}
}

func TestValidateFlow_BadSchemaPath(t *testing.T) {
	flow := &model.Flow{Name: "abc"}
	if err := ValidateFlow(flow, "does_not_exist.json"); err == nil {
		t.Errorf("expected error for missing schema file, got nil")
	}
}

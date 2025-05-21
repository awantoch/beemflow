package dsl

import "github.com/awantoch/beemflow/model"

// Lint performs semantic checks (duplicate IDs, cycles) on a flow and returns a slice of errors.
func Lint(flow *model.Flow) []error {
	// TODO: implement detailed lint rules
	return nil
}

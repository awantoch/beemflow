package docs

import _ "embed"

// BeemflowSpec is the embedded BeemFlow spec document.
//
// This is the canonical, up-to-date protocol and config spec, including event bus config and schema references.
//
//go:embed SPEC.md
var BeemflowSpec string

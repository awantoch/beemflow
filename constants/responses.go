package constants

// HTTP Response Messages
const (
	ResponseInvalidRequestBody      = "invalid request body"
	ResponseInvalidRunID            = "invalid run ID"
	ResponseRunNotFound             = "run not found"
	ResponseMissingFlowParameter    = "missing flow parameter"
	ResponseMissingFlowName         = "missing flow name"
	ResponseFailedToLoadConfig      = "failed to load config"
	ResponseFailedToGetStorage      = "failed to get storage"
	ResponseFailedToSaveRun         = "failed to save run"
	ResponseFailedToListTools       = "failed to list tools"
	ResponseFailedToGetToolManifest = "failed to get tool manifest"
	ResponseInvalidFlowSpec         = "invalid flow spec"
	ResponseRunError                = "run error"
	ResponseMissingOpenAPIField     = "missing openapi field"
	ResponseConversionFailed        = "conversion failed"
)

// Implementation Status Messages
const (
	MsgUploadFlowNotImplemented = "upload/update flow not implemented yet"
	MsgDeleteFlowNotImplemented = "delete flow not implemented yet"
)

// Error Messages for Logging
const (
	LogFailedEncodeMetadata   = "Failed to encode metadata response: %v"
	LogFailedWriteHealthCheck = "Failed to write health check response: %v"
	LogFailedWriteSpec        = "Failed to write spec response: %v"
	LogFailedEncodeJSON       = "Failed to encode JSON response"
	LogFailedWriteText        = "Failed to write text response"
	LogWriteFailed            = "w.Write failed: %v"
	LogJSONEncodeFailed       = "json.Encode failed: %v"
)

// Validation Messages
const (
	ValidationFailed = "validation failed: %v"
)

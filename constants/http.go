package constants

// HTTP Methods
const (
	HTTPMethodGET    = "GET"
	HTTPMethodPOST   = "POST"
	HTTPMethodPUT    = "PUT"
	HTTPMethodPATCH  = "PATCH"
	HTTPMethodDELETE = "DELETE"
)

// Content Types
const (
	ContentTypeJSON           = "application/json"
	ContentTypeForm           = "application/x-www-form-urlencoded"
	ContentTypeText           = "text/plain"
	ContentTypeMarkdown       = "text/markdown"
	ContentTypeTextMarkdown   = "text/markdown"
	ContentTypeTextVndMermaid = "text/vnd.mermaid"
)

// HTTP Headers
const (
	HeaderContentType   = "Content-Type"
	HeaderAuthorization = "Authorization"
	HeaderAccept        = "Accept"
)

// HTTP Status Messages
const (
	StatusOK        = "ok"
	StatusValid     = "valid"
	StatusStarted   = "STARTED"
	StatusDeleted   = "deleted"
	StatusPublished = "published"
)

// Default Values
const (
	DefaultAPIName    = "api"
	DefaultBaseURL    = "https://api.example.com"
	DefaultJSONAccept = "application/json, text/*;q=0.9, */*;q=0.8"
)

// URL Paths and Prefixes
const (
	PathRuns   = "/runs/"
	PathResume = "/resume/"
	PathFlows  = "/flows/"
	PathTools  = "/tools/"
)

// Query Parameters
const (
	QueryParamFlow = "flow"
)

// JSON Field Names - Core API Fields
const (
	FieldError   = "error"
	FieldStatus  = "status"
	FieldRunID   = "run_id"
	FieldOutputs = "outputs"
	FieldFlows   = "flows"
	FieldOn      = "on"
	FieldFlow    = "flow"
	FieldEvent   = "event"
	FieldSpec    = "spec"
	FieldTopic   = "topic"
	FieldPayload = "payload"
)

// JSON Field Names - OpenAPI Conversion (shared by HTTP and MCP)
const (
	FieldOpenAPI = "openapi"
	FieldAPIName = "api_name"
	FieldBaseURL = "base_url"
)

// Health Check Response
const (
	HealthCheckResponse = `{"status":"ok"}`
)

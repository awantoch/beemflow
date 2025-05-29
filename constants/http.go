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

// Default Values
const (
	DefaultAPIName    = "api"
	DefaultBaseURL    = "https://api.example.com"
	DefaultJSONAccept = "application/json, text/*;q=0.9, */*;q=0.8"
)

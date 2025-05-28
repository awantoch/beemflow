package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/utils"
)

// CoreAdapter handles built-in BeemFlow utilities and debugging tools.
type CoreAdapter struct{}

// ID returns the adapter ID.
func (a *CoreAdapter) ID() string {
	return "core"
}

// Execute handles various core BeemFlow tools based on the __use field.
func (a *CoreAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	use, ok := inputs["__use"].(string)
	if !ok {
		return nil, fmt.Errorf("missing __use for CoreAdapter")
	}

	switch use {
	case "core.echo":
		return a.executeEcho(ctx, inputs)
	case "core.convert_openapi":
		return a.executeConvertOpenAPI(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown core tool: %s", use)
	}
}

// executeEcho prints the 'text' field to stdout and returns inputs unchanged.
func (a *CoreAdapter) executeEcho(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if text, ok := inputs["text"].(string); ok {
		if os.Getenv("BEEMFLOW_DEBUG") != "" {
			utils.Info("%s", text)
		}
	}

	// Return inputs but filter out internal fields like __use
	result := make(map[string]any)
	for k, v := range inputs {
		if k != "__use" {
			result[k] = v
		}
	}

	return result, nil
}

// executeConvertOpenAPI converts OpenAPI specs to BeemFlow tool manifests.
func (a *CoreAdapter) executeConvertOpenAPI(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Get required inputs - can be either a JSON string or an object
	var spec map[string]any
	if openapiStr, ok := inputs["openapi"].(string); ok {
		// Parse JSON string
		if err := json.Unmarshal([]byte(openapiStr), &spec); err != nil {
			return nil, fmt.Errorf("invalid OpenAPI JSON: %w", err)
		}
	} else if openapiObj, ok := inputs["openapi"].(map[string]any); ok {
		// Use object directly
		spec = openapiObj
	} else {
		return nil, fmt.Errorf("missing required field: openapi (must be JSON string or object)")
	}

	// Get optional inputs with defaults
	apiName, _ := inputs["api_name"].(string)
	if apiName == "" {
		apiName = "api"
	}

	baseURL, _ := inputs["base_url"].(string)

	// Extract base URL from spec if not provided
	if baseURL == "" {
		if servers, ok := spec["servers"].([]any); ok && len(servers) > 0 {
			if server, ok := servers[0].(map[string]any); ok {
				if url, ok := server["url"].(string); ok {
					baseURL = url
				}
			}
		}
		if baseURL == "" {
			baseURL = "https://api.example.com"
		}
	}

	// Convert OpenAPI paths to BeemFlow tool manifests
	manifests, err := a.convertOpenAPIToManifests(spec, apiName, baseURL)
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	// Return the converted manifests
	return map[string]any{
		"manifests": manifests,
		"count":     len(manifests),
		"api_name":  apiName,
		"base_url":  baseURL,
	}, nil
}

// convertOpenAPIToManifests converts OpenAPI spec to BeemFlow tool manifests
func (a *CoreAdapter) convertOpenAPIToManifests(spec map[string]any, apiName, baseURL string) ([]map[string]any, error) {
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("no paths found in OpenAPI spec")
	}

	var manifests []map[string]any

	for path, pathItem := range paths {
		pathObj, ok := pathItem.(map[string]any)
		if !ok {
			continue
		}

		// Process each HTTP method
		for method, operation := range pathObj {
			if !a.isValidHTTPMethod(method) {
				continue
			}

			opObj, ok := operation.(map[string]any)
			if !ok {
				continue
			}

			// Generate tool name
			toolName := a.generateToolName(apiName, path, method)

			// Extract description
			description := a.extractDescription(opObj, path)

			// Extract parameters schema
			parameters := a.extractParameters(opObj, method)

			// Create manifest
			manifest := map[string]any{
				"name":        toolName,
				"description": description,
				"kind":        "task",
				"parameters":  parameters,
				"endpoint":    baseURL + path,
				"method":      strings.ToUpper(method),
				"headers": map[string]string{
					"Content-Type":  a.determineContentType(opObj, method),
					"Authorization": "Bearer $env:" + strings.ToUpper(apiName) + "_API_KEY",
				},
			}

			manifests = append(manifests, manifest)
		}
	}

	return manifests, nil
}

// Helper functions for OpenAPI conversion

func (a *CoreAdapter) isValidHTTPMethod(method string) bool {
	validMethods := map[string]bool{
		"get": true, "post": true, "put": true, "patch": true, "delete": true,
	}
	return validMethods[strings.ToLower(method)]
}

func (a *CoreAdapter) generateToolName(apiName, path, method string) string {
	// Clean path: remove leading slash, replace slashes and special chars with underscores
	cleanPath := strings.TrimPrefix(path, "/")
	cleanPath = strings.ReplaceAll(cleanPath, "/", "_")

	// Replace path parameters {param} with _by_id
	re := regexp.MustCompile(`\{[^}]+\}`)
	cleanPath = re.ReplaceAllString(cleanPath, "_by_id")

	// Remove non-alphanumeric characters except underscores
	var result strings.Builder
	for _, r := range cleanPath {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	cleanPath = result.String()

	// Remove duplicate underscores and trailing underscores
	cleanPath = strings.Trim(cleanPath, "_")
	for strings.Contains(cleanPath, "__") {
		cleanPath = strings.ReplaceAll(cleanPath, "__", "_")
	}

	return apiName + "." + cleanPath
}

func (a *CoreAdapter) extractDescription(operation map[string]any, path string) string {
	if summary, ok := operation["summary"].(string); ok && summary != "" {
		return summary
	}
	if desc, ok := operation["description"].(string); ok && desc != "" {
		return desc
	}
	return "API endpoint: " + path
}

func (a *CoreAdapter) extractParameters(operation map[string]any, method string) map[string]any {
	// For POST/PUT/PATCH, look for requestBody
	if strings.ToUpper(method) != "GET" {
		if requestBody, ok := operation["requestBody"].(map[string]any); ok {
			if content, ok := requestBody["content"].(map[string]any); ok {
				// Try application/json first
				if jsonContent, ok := content["application/json"].(map[string]any); ok {
					if schema, ok := jsonContent["schema"].(map[string]any); ok {
						return schema
					}
				}
				// Try application/x-www-form-urlencoded
				if formContent, ok := content["application/x-www-form-urlencoded"].(map[string]any); ok {
					if schema, ok := formContent["schema"].(map[string]any); ok {
						return schema
					}
				}
			}
		}
	}

	// For GET or if no requestBody, look for parameters
	if params, ok := operation["parameters"].([]any); ok && len(params) > 0 {
		properties := make(map[string]any)
		var required []string

		for _, param := range params {
			if paramObj, ok := param.(map[string]any); ok {
				if name, ok := paramObj["name"].(string); ok {
					prop := map[string]any{
						"type": "string", // Default type
					}

					if desc, ok := paramObj["description"].(string); ok {
						prop["description"] = desc
					}

					if schema, ok := paramObj["schema"].(map[string]any); ok {
						if paramType, ok := schema["type"].(string); ok {
							prop["type"] = paramType
						}
						if enum, ok := schema["enum"].([]any); ok {
							prop["enum"] = enum
						}
					}

					properties[name] = prop

					if req, ok := paramObj["required"].(bool); ok && req {
						required = append(required, name)
					}
				}
			}
		}

		return map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   required,
		}
	}

	// Default empty schema
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (a *CoreAdapter) determineContentType(operation map[string]any, method string) string {
	if strings.ToUpper(method) == "GET" {
		return "application/json"
	}

	// Check if requestBody specifies form data
	if requestBody, ok := operation["requestBody"].(map[string]any); ok {
		if content, ok := requestBody["content"].(map[string]any); ok {
			if _, hasForm := content["application/x-www-form-urlencoded"]; hasForm {
				return "application/x-www-form-urlencoded"
			}
		}
	}

	return "application/json"
}

func (a *CoreAdapter) Manifest() *registry.ToolManifest {
	return nil
}

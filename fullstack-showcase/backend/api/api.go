// Package api exposes the embedded OpenAPI specification.
package api

import _ "embed"

// Spec is the embedded OpenAPI 3.x JSON document for this service.
//
//go:embed openapi.json
var Spec []byte

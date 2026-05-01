// Package api exposes the embedded OpenAPI specification.
//
// The spec lives next to this file so it is available to the handler package
// without a runtime filesystem read. Build it once into the binary; reload
// only by rebuilding.
package api

import _ "embed"

// Spec is the embedded OpenAPI 3.x JSON document for this service.
//
//go:embed openapi.json
var Spec []byte

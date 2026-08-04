// Package api exposes the embedded OpenAPI specification.
//
// The spec lives next to this file so it is available to the handler package
// without a runtime filesystem read. Build it once into the binary; reload
// only by rebuilding.
package api

import _ "embed"

// Adding your first endpoint — worked example (comments only; nothing here ships).
//
// openapi.json is embedded into the binary below and served at /api/openapi.json,
// so anything written into it reaches production. Keep it minimal and real: put
// examples here, in comments, never in the spec itself.
//
// Add a path to the "paths" map in openapi.json, e.g.:
//
//	"/things": {
//	  "get": {
//	    "operationId": "listThings",
//	    "summary": "List things",
//	    "responses": {"200": {"description": "OK",
//	      "content": {"application/json": {"schema": {
//	        "type": "object",
//	        "required": ["things"],
//	        "properties": {"things": {"type": "array", "items": {
//	          "type": "object",
//	          "properties": {"id": {"type": "string"},
//	                         "name": {"type": "string"}}}}}}}}}}
//	  },
//	  "post": {
//	    "operationId": "createThing",
//	    "requestBody": {"required": true, "content": {"application/json":
//	      {"schema": {"type": "object", "required": ["name"],
//	        "properties": {"name": {"type": "string"}}}}}},
//	    "responses": {"201": {"description": "Created",
//	      "content": {"application/json": {"schema": {"type": "object",
//	        "properties": {"id": {"type": "string"},
//	                       "name": {"type": "string"}}}}}}}
//	  }
//	}
//
// Five rules this demonstrates:
//  1. The path key is RELATIVE to servers[0].url ("/api/v1"), so "/things" is
//     served at /api/v1/things. Writing "/api/v1/things" here double-prefixes and
//     every agent tool call 404s while the browser UI keeps working.
//  2. Never list /health — it would register a phantom workspace tool.
//  3. Each operationId becomes an MCP tool named workspace_<toolKey>_<operationId>
//     (toolKey derives from "name" in moses-app.config.json), so listThings is
//     callable by agents as workspace_<toolKey>_listThings.
//  4. Never put a tenant UUID in a response schema (CHAT-w6gt).
//  5. The list response is an OBJECT ({"things": [...]}), never a bare array.
//     That is what the handler encodes (map[string]any{"things": things}) and
//     what the frontend types (fetchAPI<{ things: Thing[] }>); all three layers
//     must agree or the generated MCP tool returns something the caller cannot
//     read. Schemas are INLINED above so nothing dangles — if you prefer
//     {"$ref": "#/components/schemas/Thing"} you must also define that schema
//     under components.schemas in openapi.json, or the served spec is invalid.
//
// Then register the matching route in cmd/server/demo_routes.go — the worked
// route example is real, CI-compiled code in cmd/server/example_test.go. The
// spec and the router must agree, and main_test.go locks that.
//
// Spec is the embedded OpenAPI 3.x JSON document for this service.
//
//go:embed openapi.json
var Spec []byte

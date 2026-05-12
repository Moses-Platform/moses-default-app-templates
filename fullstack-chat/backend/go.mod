module github.com/moses-platform/fullstack-chat

go 1.24

require (
	github.com/lib/pq v1.10.9
	github.com/moses-platform/moses-templates-shared/mosesproxy-go v0.0.0-00010101000000-000000000000
)

// Resolved via the repo-root go.work workspace file
// (../../shared/mosesproxy-go). The pseudo-version above is the Go
// idiomatic placeholder for "version supplied by replace / workspace".
// `go.work` covers builds inside the templates monorepo; templates that
// fork this code should either keep the workspace or add a `replace`
// directive pointing at the vendored shared package.
replace github.com/moses-platform/moses-templates-shared/mosesproxy-go => ../../shared/mosesproxy-go

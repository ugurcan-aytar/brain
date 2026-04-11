// Package version exposes the brain binary's current version string.
//
// Current is a var (not const) because goreleaser overrides it at build
// time via:
//
//	-ldflags="-X github.com/ugurcan-aytar/brain/internal/version.Current=1.2.3"
//
// Local `go build` picks up the baked-in default below; release builds
// always inject the tag.
package version

var Current = "0.1.7"

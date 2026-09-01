// Package migrations embeds the .sql files into the compiled binary, so that
// "go run ./cmd/api" finds them no matter which directory it is started from.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

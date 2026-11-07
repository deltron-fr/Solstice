package migrations

import "embed"

// Files contains the versioned SQL migrations applied by the application.
//
//go:embed *.sql
var Files embed.FS

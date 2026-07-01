// Package migrations embeds the SQL migration files that define the
// Verdex relational schema, so they ship inside the compiled binary
// rather than needing to be deployed as loose files alongside it.
package migrations

import "embed"

// FS embeds every migration file in this directory. Pass FS and "."
// to persistence.NewMigrator to run these migrations against a live
// database.
//
//go:embed *.sql
var FS embed.FS

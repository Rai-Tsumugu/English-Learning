package db

import "embed"

// MigrationsFS holds the embedded goose SQL migrations.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

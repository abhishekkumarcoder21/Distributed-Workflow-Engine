package migrations

import "embed"

// FS contains the embedded migration SQL files.
//
//go:embed *.up.sql
var FS embed.FS

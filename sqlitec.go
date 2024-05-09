//go:build sqlitec
// +build sqlitec

package greener

import (
	_ "github.com/mattn/go-sqlite3"
)

var SqlDriver = "sqlite3"
var sqlitePragmas = map[string]string{
	"_journal_mode": "WAL",
	"_busy_timeout": "5000",
	"_synchronous":  "NORMAL",
	"_cache_size":   "1000000000", // 1GB
	"_foreign_keys": "true",
	"temp_store":    "memory",
	"_txlock":       "immediate",
	// "cache":         "shared",
}

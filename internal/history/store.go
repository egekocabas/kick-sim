// Package history persists generated events and delivery attempts in SQLite.
package history

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/egekocabas/kick-sim/internal/config"
	_ "modernc.org/sqlite"
)

const schemaVersion = 1

// Store owns the SQLite connection and retention settings for delivery history.
type Store struct {
	db       *sql.DB
	path     string
	settings config.History
}

// OpenExisting does not create an absent database. An existing database is
// integrity-checked and migrated before it is returned.
func OpenExisting(path string, settings config.History) (*Store, error) {
	if !settings.Enabled {
		return &Store{path: path, settings: settings}, nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return &Store{path: path, settings: settings}, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect history database: %w", err)
	}
	return Open(path, settings)
}

// Open creates or opens a history database and prepares its schema.
func Open(path string, settings config.History) (*Store, error) {
	if !settings.Enabled {
		return &Store{path: path, settings: settings}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create runtime directory: %w", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve history database path: %w", err)
	}
	info, statErr := os.Stat(absolute)
	existed := statErr == nil && info.Size() > 0
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect history database: %w", statErr)
	}

	databasePath := filepath.ToSlash(absolute)
	if filepath.VolumeName(absolute) != "" && !strings.HasPrefix(databasePath, "/") {
		databasePath = "/" + databasePath
	}
	databaseURL := &url.URL{Scheme: "file", Path: databasePath}
	query := databaseURL.Query()
	query.Set("_foreign_keys", "on")
	query.Set("_journal_mode", "WAL")
	query.Set("_busy_timeout", "5000")
	query.Set("_synchronous", "NORMAL")
	databaseURL.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return nil, fmt.Errorf("open history database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	store := &Store{db: db, path: absolute, settings: settings}
	if err := store.prepare(existed); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close releases the database connection; it is safe for disabled stores.
func (store *Store) Close() error {
	if store == nil || store.db == nil {
		return nil
	}
	return store.db.Close()
}

// Enabled reports whether the store has an active history database.
func (store *Store) Enabled() bool {
	return store != nil && store.db != nil && store.settings.Enabled
}

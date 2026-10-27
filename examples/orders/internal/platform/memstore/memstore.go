// Package memstore is an in-memory, transactional key-value store used by the
// example so that persistence code can be compiled and tested without a real
// database. It is deliberately small: tables of JSON rows, serializable
// transactions, and failure injection for commit-path tests.
//
// It stands in for whatever database a service chooses. Production adapters
// replace it with real transactions (SQL, document store, ...) but keep the
// same contract: everything written inside one InTx call commits or rolls back
// together.
package memstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// ErrNotFound is returned when a key does not exist in a table.
var ErrNotFound = errors.New("memstore: not found")

// ErrAlreadyExists is returned by Insert when the key is already present.
var ErrAlreadyExists = errors.New("memstore: already exists")

// DB holds tables of encoded rows. It is safe for concurrent use; transactions
// are serialized.
type DB struct {
	mu         sync.Mutex
	tables     map[string]map[string][]byte
	failCommit error
}

// New returns an empty DB.
func New() *DB {
	return &DB{tables: make(map[string]map[string][]byte)}
}

// FailNextCommit makes the next transaction fail at commit time with err,
// after its callback has run successfully. Tests use it to prove that pending
// work survives a failed commit.
func (db *DB) FailNextCommit(err error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.failCommit = err
}

// InTx runs fn inside a serializable transaction. All writes made through tx
// become visible together when fn returns nil and the commit succeeds;
// otherwise none of them do.
func (db *DB) InTx(ctx context.Context, fn func(tx *Tx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	db.mu.Lock()
	defer db.mu.Unlock()

	tx := &Tx{snapshot: db.tables, writes: make(map[string]map[string][]byte)}
	if err := fn(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if db.failCommit != nil {
		err := db.failCommit
		db.failCommit = nil
		return fmt.Errorf("commit: %w", err)
	}
	for table, rows := range tx.writes {
		committed := maps.Clone(db.tables[table])
		if committed == nil {
			committed = make(map[string][]byte)
		}
		maps.Copy(committed, rows)
		db.tables[table] = committed
	}
	return nil
}

// Tx is a transaction handle. It is valid only inside the InTx callback.
type Tx struct {
	snapshot map[string]map[string][]byte
	writes   map[string]map[string][]byte
}

// Get decodes the row stored under key into dst.
func (tx *Tx) Get(table, key string, dst any) error {
	raw, ok := tx.lookup(table, key)
	if !ok {
		return ErrNotFound
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode %s/%s: %w", table, key, err)
	}
	return nil
}

// Exists reports whether key is present in table.
func (tx *Tx) Exists(table, key string) bool {
	_, ok := tx.lookup(table, key)
	return ok
}

// Insert stores a new row and fails with ErrAlreadyExists if key is present.
func (tx *Tx) Insert(table, key string, row any) error {
	if tx.Exists(table, key) {
		return ErrAlreadyExists
	}
	return tx.Put(table, key, row)
}

// Put stores row under key, replacing any existing row.
func (tx *Tx) Put(table, key string, row any) error {
	raw, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("encode %s/%s: %w", table, key, err)
	}
	if tx.writes[table] == nil {
		tx.writes[table] = make(map[string][]byte)
	}
	tx.writes[table][key] = raw
	return nil
}

// Keys returns the keys of table in ascending order, including uncommitted
// writes made by this transaction.
func (tx *Tx) Keys(table string) []string {
	keys := make(map[string]struct{})
	for key := range tx.snapshot[table] {
		keys[key] = struct{}{}
	}
	for key := range tx.writes[table] {
		keys[key] = struct{}{}
	}
	return slices.Sorted(maps.Keys(keys))
}

func (tx *Tx) lookup(table, key string) ([]byte, bool) {
	if value, ok := tx.writes[table][key]; ok {
		return value, true
	}
	value, ok := tx.snapshot[table][key]
	return value, ok
}

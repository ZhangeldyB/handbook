package memstore

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type row struct{ Value string }

func TestInTxCommitsAllWritesTogether(t *testing.T) {
	db := New()

	err := db.InTx(t.Context(), func(tx *Tx) error {
		if err := tx.Insert("a", "1", row{Value: "x"}); err != nil {
			return err
		}
		return tx.Insert("b", "1", row{Value: "y"})
	})
	if err != nil {
		t.Fatalf("InTx() error = %v", err)
	}

	err = db.InTx(t.Context(), func(tx *Tx) error {
		var got row
		if err := tx.Get("b", "1", &got); err != nil {
			return err
		}
		if got.Value != "y" {
			t.Errorf("Get() = %q, want %q", got.Value, "y")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read InTx() error = %v", err)
	}
}

func TestInTxRollsBackOnCallbackError(t *testing.T) {
	db := New()
	errBoom := errors.New("boom")

	err := db.InTx(t.Context(), func(tx *Tx) error {
		if err := tx.Insert("a", "1", row{}); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx() error = %v, want %v", err, errBoom)
	}
	assertMissing(t, db, "a", "1")
}

func TestFailNextCommitRollsBack(t *testing.T) {
	db := New()
	errCommit := errors.New("unavailable")
	db.FailNextCommit(errCommit)

	err := db.InTx(t.Context(), func(tx *Tx) error { return tx.Insert("a", "1", row{}) })
	if !errors.Is(err, errCommit) {
		t.Fatalf("InTx() error = %v, want %v", err, errCommit)
	}
	assertMissing(t, db, "a", "1")
}

func TestInsertRejectsDuplicateKey(t *testing.T) {
	db := New()
	_ = db.InTx(t.Context(), func(tx *Tx) error { return tx.Insert("a", "1", row{}) })

	err := db.InTx(t.Context(), func(tx *Tx) error { return tx.Insert("a", "1", row{}) })
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Insert() error = %v, want %v", err, ErrAlreadyExists)
	}
}

func TestKeysAreSortedAndIncludeOwnWrites(t *testing.T) {
	db := New()
	_ = db.InTx(t.Context(), func(tx *Tx) error { return tx.Put("a", "2", row{}) })

	_ = db.InTx(t.Context(), func(tx *Tx) error {
		if err := tx.Put("a", "1", row{}); err != nil {
			return err
		}
		if got, want := tx.Keys("a"), []string{"1", "2"}; !slices.Equal(got, want) {
			t.Errorf("Keys() = %v, want %v", got, want)
		}
		return nil
	})
}

func TestInTxHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := New().InTx(ctx, func(*Tx) error {
		t.Fatal("callback ran with a canceled context")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTx() error = %v, want %v", err, context.Canceled)
	}
}

func assertMissing(t *testing.T, db *DB, table, key string) {
	t.Helper()
	_ = db.InTx(t.Context(), func(tx *Tx) error {
		if tx.Exists(table, key) {
			t.Errorf("%s/%s exists after rollback", table, key)
		}
		return nil
	})
}

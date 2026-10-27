package aggregate

import (
	"errors"
	"slices"
	"testing"
)

func TestNewForCreateIsDirtyAndNew(t *testing.T) {
	tracker := NewForCreate[string]()

	if !tracker.IsNew() || !tracker.IsDirty() {
		t.Fatalf("IsNew() = %v, IsDirty() = %v, want true, true", tracker.IsNew(), tracker.IsDirty())
	}
}

func TestNewForReconstituteIsClean(t *testing.T) {
	tracker := NewForReconstitute[string](7)

	if tracker.IsNew() || tracker.IsDirty() {
		t.Fatalf("IsNew() = %v, IsDirty() = %v, want false, false", tracker.IsNew(), tracker.IsDirty())
	}
	if got, want := tracker.Version(), int64(7); got != want {
		t.Fatalf("Version() = %d, want %d", got, want)
	}
}

func TestPersistSuccessAcknowledgesCapturedWork(t *testing.T) {
	tracker := NewForReconstitute[string](3)
	tracker.Record("placed")

	var got Pending[string]
	err := tracker.Persist(func(p Pending[string]) error {
		got = p
		return nil
	})
	if err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	if got.IsNew || got.ExpectedVersion != 3 || got.NextVersion != 4 || !slices.Equal(got.Events, []string{"placed"}) {
		t.Fatalf("pending = %+v, want version 3->4 with one event", got)
	}
	if tracker.IsDirty() || tracker.EventCount() != 0 || tracker.Version() != 4 {
		t.Fatalf("after success: dirty=%v events=%d version=%d, want clean, 0, 4",
			tracker.IsDirty(), tracker.EventCount(), tracker.Version())
	}
}

func TestPersistFailureRetainsWorkForRetry(t *testing.T) {
	tracker := NewForCreate[string]()
	tracker.Record("created")
	errCommit := errors.New("commit failed")

	if err := tracker.Persist(func(Pending[string]) error { return errCommit }); !errors.Is(err, errCommit) {
		t.Fatalf("Persist() error = %v, want %v", err, errCommit)
	}
	if !tracker.IsNew() || !tracker.IsDirty() || tracker.EventCount() != 1 {
		t.Fatalf("failed commit acknowledged work: new=%v dirty=%v events=%d",
			tracker.IsNew(), tracker.IsDirty(), tracker.EventCount())
	}

	var retried Pending[string]
	if err := tracker.Persist(func(p Pending[string]) error { retried = p; return nil }); err != nil {
		t.Fatalf("retry Persist() error = %v", err)
	}
	if !retried.IsNew || !slices.Equal(retried.Events, []string{"created"}) {
		t.Fatalf("retry pending = %+v, want the same new aggregate and event", retried)
	}
}

func TestPersistOnCleanTrackerDoesNotCallApply(t *testing.T) {
	tracker := NewForReconstitute[string](1)

	err := tracker.Persist(func(Pending[string]) error {
		t.Fatal("apply called for a clean aggregate")
		return nil
	})
	if err != nil {
		t.Fatalf("Persist() error = %v", err)
	}
}

func TestWorkRecordedDuringApplyStaysPending(t *testing.T) {
	tracker := NewForReconstitute[string](1)
	tracker.Record("first")

	err := tracker.Persist(func(Pending[string]) error {
		tracker.Record("second")
		return nil
	})
	if err != nil {
		t.Fatalf("Persist() error = %v", err)
	}
	if !tracker.IsDirty() || tracker.EventCount() != 1 {
		t.Fatalf("dirty=%v events=%d, want newer event still pending", tracker.IsDirty(), tracker.EventCount())
	}
}

func TestStaleAckIsIgnored(t *testing.T) {
	tracker := NewForReconstitute[string](1)
	tracker.Record("first")

	_, staleAck, _ := tracker.Begin()
	_, ack, _ := tracker.Begin()
	ack()
	tracker.Record("second")
	staleAck()

	if tracker.Version() != 2 || tracker.EventCount() != 1 {
		t.Fatalf("version=%d events=%d, want stale ack ignored", tracker.Version(), tracker.EventCount())
	}
}

func TestZeroTrackerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("zero Tracker did not panic")
		}
	}()
	var tracker Tracker[string]
	tracker.Changed()
}

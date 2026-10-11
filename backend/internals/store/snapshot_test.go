package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.snapshot")

	data := map[string]string{"a": "1", "b": "2", "c": "3"}

	if err := Save(path, data); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 3 {
		t.Errorf("want 3 entries, got %d", len(loaded))
	}

	for k, want := range data {
		if got := loaded[k]; got != want {
			t.Errorf("key %q: want %q, got %q", k, want, got)
		}
	}
}

func TestSnapshotLoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.snapshot")

	data, err := Load(path)

	if err != nil {
		t.Fatalf("want nil error for missing file, got %v", err)
	}

	if data != nil {
		t.Errorf("want nil error for missing file, got %v", data)
	}

}

func TestSnapshotReplacesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.snapshot")

	if err := Save(path, map[string]string{"old": "data"}); err != nil {
		t.Fatal(err)
	}

	if err := Save(path, map[string]string{"new": "data"}); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 1 || loaded["new"] != "data" {
		t.Errorf("want {new:data} , got %v ", loaded)
	}

	if _, stillThere := loaded["old"]; stillThere {
		t.Errorf("old key should be gone after replace")
	}

	// Verify no .tmp file was left behind.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file should not exist after rename")
	}
}

func TestSnapshotAndRecover(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "test.wal")

	s, err := New(walPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := s.Set(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	// Force a snapshot.
	if err := s.Snapshot(); err != nil {
		t.Fatal(err)
	}

	// ✅ NEW: verify the snapshot file was actually created.
	snapPath := strings.TrimSuffix(walPath, filepath.Ext(walPath)) + ".snapshot"
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("snapshot file missing after Snapshot(): %v", err)
	}

	// ✅ NEW: verify the WAL was truncated to 0.
	walInfo, err := os.Stat(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if walInfo.Size() != 0 {
		t.Errorf("WAL should be empty after snapshot, got %d bytes", walInfo.Size())
	}

	s.Close()

	// Reopen and verify.
	s2, err := New(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	for i := 0; i < 10; i++ {
		v, err := s2.Get(fmt.Sprintf("k%d", i))
		if err != nil {
			t.Errorf("k%d: %v", i, err)
		}
		if v != fmt.Sprintf("v%d", i) {
			t.Errorf("k%d: want v%d, got %s", i, i, v)
		}
	}
}

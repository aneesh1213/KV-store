package store

import (
	"os"
	"path/filepath"
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

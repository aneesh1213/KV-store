package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// test 1
func TestWalReplayClean(t *testing.T) {
	// create a temp dir path
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}

	// append 2 records
	if err := w.Append([]byte(`{"op":"SET","key":"a","value":"1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Append([]byte(`{"op":"SET","key":"b","value":"2"}`)); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen

	w2, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}

	defer w2.Close()

	// step 6

	count := 0
	offset, err := w2.Replay(func(line []byte) error {
		count++
		return nil
	})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Errorf("want fn called 2 times, got %d", count)
	}

	if offset != info.Size() {
		t.Errorf("want offset == file size (%d), got %d", info.Size(), offset)
	}
}

// test 2
func TestWalReplayPartialRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Append([]byte(`{"op":"SET","key":"a","value":"1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Append([]byte(`{"op":"SET","key":"b","value":"2"}`)); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Simulate crash mid-write: append raw bytes with NO trailing newline.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)

	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Write([]byte(`{"op":"SE`)); err != nil { // no '\n'!
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen

	w2, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}

	defer w2.Close()

	// Replay

	count := 0
	offset, err := w2.Replay(func(line []byte) error {
		count++
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Errorf("want fn called 2 times, got %d", count)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if offset >= info.Size() {
		t.Errorf("want offset < file size, got offset=%d size=%d", offset, info.Size())
	}

	// Truncate away the partial record.
	if err := w2.Truncate(offset); err != nil {
		t.Fatal(err)
	}

	info2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info2.Size() != offset {
		t.Errorf("after truncate want size %d, got %d", offset, info2.Size())
	}

}

// test 3
func TestWalReplayRejectsBadLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}

	// 1 good, 1 bad, 1 good — all get appended via WAL (they're all valid
	// line-writes as far as WAL is concerned; WAL doesn't parse JSON).
	if err := w.Append([]byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Append([]byte(`not json at all`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Append([]byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	w2, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	// Replay with a fn that rejects the bad line.
	sentinel := errors.New("bad line")
	count := 0
	offset, err := w2.Replay(func(line []byte) error {
		if string(line) == "not json at all" {
			return sentinel
		}
		count++
		return nil
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("want sentinel error, got %v", err)
	}
	if count != 1 {
		t.Errorf("want fn called 1 time, got %d", count)
	}
	const firstRecordSize = int64(len(`{"ok":true}`) + 1) // +1 for '\n'
	if offset != firstRecordSize {
		t.Errorf("want offset %d, got %d", firstRecordSize, offset)
	}

}

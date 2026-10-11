package store

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var NotFound = errors.New("key not found")

const defaultNumShards = 32

// struct for the shard

type Shard struct {
	mu   sync.RWMutex
	data map[string]string
}

// need a map for the store strcut

type Store struct {
	shards              []*Shard
	totalShards         uint32
	wal                 *WAL
	snapshotPath        string
	writesMu            sync.RWMutex
	writesSinceSnapshot int64
	snapshotThreshold   int64
}

// returns a emoty store ready to use
func New(path string) (*Store, error) {
	return NewWithShards(path, defaultNumShards)
}

func NewWithShards(path string, n uint32) (*Store, error) {
	// 1. Derive snapshot path from WAL path.
	//    If path = "./data/kvstore.wal", then snapshotPath = "./data/kvstore.snapshot".
	//    HINT: strings.TrimSuffix(path, ".wal") + ".snapshot"
	//    Or use filepath: replace the extension.

	snapshotPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".snapshot"

	// step 1 creating the shards

	s := &Store{
		shards:            make([]*Shard, n),
		totalShards:       n,
		snapshotPath:      snapshotPath,
		snapshotThreshold: 10000,
	}

	// iterate over the shards

	for i := range s.shards {
		s.shards[i] = &Shard{
			data: make(map[string]string),
		}
	}

	// before loading a wal load a snapshot first

	snap, err := Load(snapshotPath)
	if err != nil {
		return nil, err
	}

	if snap != nil {
		// populate the shards directly (no wal, no counter)
		for k, v := range snap {
			sh := s.getShard(k)
			sh.mu.Lock()
			sh.data[k] = v
			sh.mu.Unlock()
		}
	}

	// open wal and attach to it to store
	wal, err := OpenWAL(path)
	if err != nil {
		return nil, err
	}

	//step 3 attach it
	s.wal = wal

	// step 4:replay

	offset, replayErr := wal.Replay(func(line []byte) error {
		rec, derr := decodeRecord(line)
		if derr != nil {
			return derr
		}

		s.applyRecord(rec)
		return nil
	})

	if replayErr != nil {
		fmt.Printf("wal replay stopped early: %v\n", replayErr)
	}

	// Step 5: if Replay found a bad tail, truncate to the last good offset.
	// Note: err is non-nil here if a bad record was found — that's expected
	// and we want to continue anyway (forgiving mode). Log it.

	fileInfo, statErr := os.Stat(path)
	if statErr != nil {
		return nil, statErr
	}

	if fileInfo.Size() > offset {
		if err := wal.Truncate(offset); err != nil {
			return nil, err
		}
	}
	return s, nil

}

func (s *Store) getShard(key string) *Shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return s.shards[h.Sum32()%s.totalShards]
}

// applyrecord func for applying the data in the shard

func (s *Store) applyRecord(r record) {
	sh := s.getShard(r.Key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	switch r.Op {
	case "SET":
		sh.data[r.Key] = r.Value
	case "DELETE":
		delete(sh.data, r.Key)
	}

}

// set uses to set the key and value

func (s *Store) Set(key, value string) error {

	s.writesMu.RLock()
	defer s.writesMu.RUnlock()

	// step 1 : encode the record first to add in the wal
	encRec, err := encodeRecord(record{Op: "SET", Key: key, Value: value})
	if err != nil {
		return err
	}

	if err := s.wal.Append(encRec); err != nil {
		return err
	}

	sh := s.getShard(key)
	sh.mu.Lock()
	sh.data[key] = value
	sh.mu.Unlock()

	// 3. Increment counter. If it hit the threshold, trigger a snapshot
	//    in a background goroutine so we don't block the caller.
	n := atomic.AddInt64(&s.writesSinceSnapshot, 1)
	if n >= s.snapshotThreshold {
		go s.Snapshot()
	}

	return nil
}

// get returns the key from the store itself

func (s *Store) Get(key string) (string, error) {
	sh := s.getShard(key)
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	val, ok := sh.data[key]
	if !ok {
		return "", NotFound
	}

	return val, nil
}

func (s *Store) Delete(key string) error {

	s.writesMu.RLock()
	defer s.writesMu.RUnlock()

	encRec, err := encodeRecord(record{Op: "DELETE", Key: key})
	if err != nil {
		return err
	}

	if err := s.wal.Append(encRec); err != nil {
		return err
	}

	sh := s.getShard(key)
	sh.mu.Lock()
	delete(sh.data, key)
	sh.mu.Unlock()

	n := atomic.AddInt64(&s.writesSinceSnapshot, 1)
	if n >= s.snapshotThreshold {
		go s.Snapshot()
	}

	return nil
}

func (s *Store) Len() int {
	total := 0
	for _, sh := range s.shards {
		sh.mu.RLock()
		total += len(sh.data)
		sh.mu.RUnlock()
	}
	return total
}

func (s *Store) Close() error {
	return s.wal.Close()
}

func (s *Store) snapshotNow() error {
	//step 1 :Aquire write lock - blocks all set/delete for the duration
	s.writesMu.Lock()
	defer s.writesMu.Unlock()

	// Re-check: if the counter is 0, someone already snapshotted and
	// there's nothing new to capture.
	if atomic.LoadInt64(&s.writesSinceSnapshot) == 0 {
		return nil
	}
	// 2. Copy memory. Since we hold writesMu, no writes are in flight,
	//    but we still need shard RLocks because Get can be running.

	snapshot := make(map[string]string)

	for _, sh := range s.shards {
		sh.mu.RLock()
		for k, v := range sh.data {
			snapshot[k] = v
		}
		sh.mu.RUnlock()
	}

	// 3. Save to disk (atomic — snapshot.go handles the tmp/rename dance).

	if err := Save(s.snapshotPath, snapshot); err != nil {
		return err
	}

	// 4. Now truncate the WAL. Safe because writesMu blocks new writes.
	if err := s.wal.Truncate(0); err != nil {
		return err
	}

	// 5. Reset counter.
	atomic.StoreInt64(&s.writesSinceSnapshot, 0)

	return nil
}

// Snapshot forces an immediate snapshot. Used on graceful shutdown.
func (s *Store) Snapshot() error {
	return s.snapshotNow()
}

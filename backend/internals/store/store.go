package store

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"sync"
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
	shards      []*Shard
	totalShards uint32
	wal         *WAL
}

// returns a emoty store ready to use
func New(path string) (*Store, error) {
	return NewWithShards(path, defaultNumShards)
}

func NewWithShards(path string, n uint32) (*Store, error) {
	if n == 0 {
		n = 1
	}

	// step 1 creating the shards

	s := &Store{
		shards:      make([]*Shard, n),
		totalShards: n,
	}

	// iterate over the shards

	for i := range s.shards {
		s.shards[i] = &Shard{
			data: make(map[string]string),
		}
	}

	// step 2 add wal to it
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
	defer sh.mu.Unlock()
	sh.data[key] = value
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
	encRec, err := encodeRecord(record{Op: "DELETE", Key: key})
	if err != nil {
		return nil
	}

	if err := s.wal.Append(encRec); err != nil {
		return err
	}

	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	delete(sh.data, key)
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

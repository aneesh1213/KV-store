package store

import (
	"errors"
	"hash/fnv"
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
}

// returns a emoty store ready to use
func New() *Store {
	return NewWithShards(defaultNumShards)
}

func NewWithShards(n uint32) *Store {
	if n == 0 {
		n = 1
	}

	// create store object with shards\

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

	return s
}

func (s *Store) getShard(key string) *Shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return s.shards[h.Sum32()%s.totalShards]
}

// set uses to set the key and value

func (s *Store) Set(key, value string) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.data[key] = value
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

func (s *Store) Delete(key string) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	delete(sh.data, key)
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

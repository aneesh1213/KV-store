package store

import (
	"errors"
	"sync"
)

var NotFound = errors.New("key not found")

// need a map for the store strcut

type Store struct {
	mu sync.RWMutex
	data map[string]string
}

// returns a emoty store ready to use
func New() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// set uses to set the key and value

func (s *Store) Set(key, value string) {
	s.mu.Lock();
	defer s.mu.Unlock()
	s.data[key] = value
}

// get returns the key from the store itself

func (s *Store) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	if !ok {
		return "", NotFound
	}

	return val, nil
}


func (s *Store) Delete (key string){
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key);
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

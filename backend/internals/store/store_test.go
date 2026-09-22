package store

import (
	"fmt"
	"sync"
	"testing"
)


func TestConcurrentWrites(t *testing.T){
	s := New()

	var wg sync.WaitGroup

	for i:=0;i<100;i++ {
		wg.Add(1)
		go func(n int){
			defer wg.Done();
			key := fmt.Sprintf("key-%d", n)
			s.Set(key, "value")
		}(i)
	}

	wg.Wait()
	if s.Len() != 100 {
		t.Fatalf("expected 100 keys, got %d", s.Len())
	}
}
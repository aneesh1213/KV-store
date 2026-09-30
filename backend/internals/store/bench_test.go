package store

import (
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkSet(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.wal")
	s, err := New(path)
	if err != nil {
		fmt.Print("err come in test bench is: ", err)
	}

	defer s.Close()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%1000)
			s.Set(key, "value")
			i++
		}
	})
}

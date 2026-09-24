package store

import (
	"fmt"
	"testing"
)

func BenchmarkSet(b *testing.B) {
	s := New()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%1000)
			s.Set(key, "value")
			i++
		}
	})
}

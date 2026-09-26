package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aneesh1213/kvstore/backend/internals/store"
)

func main() {
	s := store.New()

	if len(os.Args) > 1 && os.Args[1] == "read" {
		// READ mode: only look for the keys, don't write them
		fmt.Println("=== READ mode ===")
		v1, err := s.Get("user:1")
		if errors.Is(err, store.NotFound) {
			fmt.Println("user:1 -> NOT FOUND")
		} else {
			fmt.Println("user:1 =", v1)
		}
		fmt.Println("total keys:", s.Len())
		return
	}

	// WRITE mode (default): insert keys, then hold so you can Ctrl+C
	fmt.Println("=== WRITE mode ===")
	s.Set("user:1", "alice")
	s.Set("user:2", "bob")
	fmt.Println("wrote user:1, user:2")
	fmt.Println("total keys:", s.Len())
	fmt.Println()
	fmt.Println("Now Ctrl+C this process (or wait 30s for it to exit).")
	time.Sleep(30 * time.Second)
}

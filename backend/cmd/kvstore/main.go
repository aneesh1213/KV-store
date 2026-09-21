package main

import (
	"errors"
	"fmt"

	"github.com/aneesh1213/kvstore/backend/internals/store"
)


func main(){
	fmt.Println("start of the kv store")

	s := store.New();

	s.Set("name", "aneesh")
    s.Set("language", "go")

	    v, err := s.Get("name")
    if err != nil {
        fmt.Println("unexpected error:", err)
        return
    }
    fmt.Println("name =", v)


	_, err = s.Get("vikas");
	if errors.Is(err, store.NotFound){
		fmt.Println("missing key correctly reported as not found")
	}

	
    s.Delete("language")
    fmt.Println("keys after delete:", s.Len())
}
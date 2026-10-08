package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aneesh1213/kvstore/backend/internals/api"
	"github.com/aneesh1213/kvstore/backend/internals/store"
)

func main() {

	// step 1 : open the store
	s, err := store.New("./data/kvstore.wal")

	if err != nil {
		log.Fatalf("failed to open store:", err)
	}

	defer s.Close()

	// step 2 : build the api server on 8080

	srv := api.New(s, ":8080")

	// step 3 : start HTTP in goroutine so that main can wait for the signals

	go func() {
		fmt.Println("listening on 8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	// step 4 : wait for cntr + c or SIGTERM

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Print("shutting down")

	// step 5: give 5 seconds to inflight requests to finish, then shut down
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.ShutDown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	// defer s.close runs now
	fmt.Print("goodbye")

}

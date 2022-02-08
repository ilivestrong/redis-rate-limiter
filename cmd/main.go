package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ilivestrong/rate-limit-poc/internal"
)

var rc internal.RedisClient

func main() {

	rc = internal.RedisClient{
		Client: internal.New(),
	}
	defer rc.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/store", storeHandler)
	mux.Handle("/get", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		val, _ := rc.Get(ctx, "name")

		select {
		case <-ctx.Done():
			fmt.Println("Redis server didn't respond in time")
		default:
			fmt.Fprint(rw, val)
		}
	}))

	http.ListenAndServe(":8000", mux)
}

func storeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rc.Store(ctx, "name", "Deepak")
	select {
	case <-ctx.Done():
		fmt.Println("Redis server didn't perform write in due time")
	default:
		fmt.Fprint(w, "Value written!!!")

	}
}

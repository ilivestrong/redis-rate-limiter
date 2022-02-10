package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis_rate/v9"
	"github.com/ilivestrong/rate-limit-poc/internal"
	"github.com/ilivestrong/rate-limit-poc/internal/limiters"
	"github.com/joho/godotenv"
)

var rc internal.RedisClient
var key = "localhost"
var ErrRedisGetExpired = errors.New("redis server didn't respond in time")
var ErrRedisWriteExpired = errors.New("redis server didn't write in time")

func main() {
	envErr := godotenv.Load("config.env")
	if envErr != nil {
		fmt.Println("failed to load env, will work with default settings")
	}

	rc = internal.RedisClient{
		Client: internal.New(),
	}
	defer rc.Close()

	authLimiter, err := limiters.NewRedisLimiter(rc.Client, &limiters.RedisLimiterConfig{
		Ctx:  context.Background(),
		Key:  key,
		Type: limiters.Authenticate,
	})
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/store", storeHandler)
	mux.Handle("/get", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		res, _ := authLimiter()
		fmt.Println(getMessage(res))
		if res.Allowed < 1 {
			http.Error(rw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		val, _ := rc.Get(ctx, "name") // get value from Redis

		select {
		case <-ctx.Done():
			fmt.Println(ErrRedisGetExpired)
		default:
			fmt.Fprint(rw, val)
		}
	}))
	http.ListenAndServe(":8000", mux)
}

func storeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rc.Store(ctx, "name", "Deepak") // put value to Redis
	select {
	case <-ctx.Done():
		fmt.Println(ErrRedisWriteExpired)
	default:
		fmt.Fprint(w, "Value written!!!")

	}
}

func getMessage(res *redis_rate.Result) string {
	if res.Allowed < 1 {
		return fmt.Sprintf("[Rate-limiter]- [ACCESS DENIED] - Key: %s, Reason:%s\n", key, http.StatusText(http.StatusTooManyRequests))
	} else {
		return fmt.Sprintf("[Rate-limiter]- [SUCCESS]- Key: %s, Allowed: %d, Remaning: %d\n", key, res.Allowed, res.Remaining)
	}
}

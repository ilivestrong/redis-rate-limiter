package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-redis/redis_rate/v9"
	"github.com/ilivestrong/rate-limit-poc/internal"
	"github.com/ilivestrong/rate-limit-poc/internal/limiters"
	"github.com/joho/godotenv"
)

type AuthRequest struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

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

	mux := http.NewServeMux()
	mux.HandleFunc("/store", storeHandler)
	mux.HandleFunc("/auth", authHandler)
	mux.Handle("/get", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
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

func authHandler(w http.ResponseWriter, r *http.Request) {
	res, _ := getAuthLimiter()()
	fmt.Println(getMessage(res))

	if res.Allowed < 1 {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}

	var authReq AuthRequest
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &authReq)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	fmt.Printf("RECEIVED AUTHENTICATE REQUEST for Type: %s, Value: %s", authReq.Type, authReq.Value)

	w.Write([]byte("OK"))
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
		return fmt.Sprintf("[Rate-limiter]- [ACCESS DENIED] - Key: %s, Reason: %s\n", key, http.StatusText(http.StatusTooManyRequests))
	} else {
		return fmt.Sprintf("[Rate-limiter]- [SUCCESS]- Key: %s, Allowed: %d, Remaning: %d\n", key, res.Allowed, res.Remaining)
	}
}

func getAuthLimiter() func() (*redis_rate.Result, error) {
	authLimiter, err := limiters.NewRedisLimiter(rc.Client, &limiters.RedisLimiterConfig{
		Ctx:  context.Background(),
		Key:  key,
		Type: limiters.Authenticate,
	})
	if err != nil {
		panic(err)
	}
	return authLimiter
}

package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	*redis.Client
}

func New() *redis.Client {
	fmt.Println("creating new client...")
	c := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	fmt.Println("New client ready...")
	return c

}

func (rc RedisClient) Store(ctx context.Context, key string, val interface{}) {
	fmt.Println("Request to store", key, val)
	time.Sleep(1 * time.Second)
	rc.Client.Set(ctx, key, val, 0)
}

func (rc RedisClient) Get(ctx context.Context, key string) (interface{}, error) {
	fmt.Println("Request to get", key)
	time.Sleep(3 * time.Second)
	return rc.Client.Get(ctx, key).Result()
}

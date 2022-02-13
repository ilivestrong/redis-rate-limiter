package internal

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	*redis.Client
}

func New() *redis.Client {
	fmt.Println("Creating new Redis client...")
	c := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	fmt.Println("Verifying Redis instance availability...")

	pingRes := c.Ping(context.Background()).Err()
	if pingRes != nil {
		panic(pingRes)
	}
	fmt.Println("New Redis client ready...")
	return c

}

func (rc RedisClient) Store(ctx context.Context, key string, val interface{}) {
	fmt.Println("Request to store", key, val)
	// time.Sleep(1 * time.Second)  // Test and Trial for expiring context timeout
	rc.Client.Set(ctx, key, val, 0)
}

func (rc RedisClient) Get(ctx context.Context, key string) (interface{}, error) {
	fmt.Println("Request to get", key)
	// time.Sleep(3 * time.Second) // Test and Trial for expiring context timeout
	return rc.Client.Get(ctx, key).Result()
}

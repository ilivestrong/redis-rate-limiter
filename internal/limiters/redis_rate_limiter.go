package limiters

import (
	"context"
	"errors"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redis_rate/v9"
)

var (
	ErrMissingRateLimiterConfig = errors.New("rate limiter config is required")
)

type LimiterType int
type RedisLimiterConfig struct {
	Ctx  context.Context
	Key  string
	Rate redis_rate.Limit
	Type LimiterType
}

const (
	Authenticate LimiterType = iota
	Otp
	Get
	Post
	Upload
)

func NewRedisLimiter(rc *redis.Client, cfg *RedisLimiterConfig) (func() (*redis_rate.Result, error), error) {
	l := redis_rate.NewLimiter(rc)
	if cfg != nil {
		return func() (*redis_rate.Result, error) {
			return l.Allow(cfg.Ctx, cfg.Key, getRate(cfg.Type))
		}, nil
	}
	return nil, ErrMissingRateLimiterConfig
}

func getRate(lt LimiterType) redis_rate.Limit {
	var rate redis_rate.Limit
	switch lt {
	case Authenticate:
		rate = redis_rate.PerHour(10) // each client can send 10 authentication requests per hour
	case Otp:
		rate = redis_rate.PerMinute(5) // each client can send 5 otp/resend requests per minute
	case Get:
		rate = redis_rate.PerHour(100) // each client can send 100 data requests per hour
	case Post:
		rate = redis_rate.PerMinute(2) // each client can send 2 post requests per minute
	}
	return rate
}

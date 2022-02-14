/* This POC rate limiter leverages "github.com/go-redis/redis_rate/v9" package, which is based
on rwz/redis-gcra and implements GCRA (aka leaky bucket) for rate limiting based on Redis.
The code requires Redis version 3.2 or newer since it relies on replicate_commands feature.
to perform rate limiting.
*/

package limiters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

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

var (
	LimiterTypes  map[LimiterType]string
	DefaultLimits map[string]int
)

func init() {
	LimiterTypes = map[LimiterType]string{
		Authenticate: "auth",
		Otp:          "otp",
		Get:          "get",
		Post:         "post",
		Upload:       "upl",
	}
	DefaultLimits = map[string]int{
		"Authenticate": 10,
		"Otp":          5,
		"Get":          100,
		"Post":         2,
	}
}

type CheckLimit func() (*redis_rate.Result, error)

// A Higher order function to generate a rate limiter as a Go middleware, providing the request handler to be wrapped
func NewRedisLimiterAsMW(rc *redis.Client, cfg *RedisLimiterConfig, next http.Handler) http.Handler {
	lim := func() (CheckLimit, error) {
		return NewRedisLimiter(rc, cfg)
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		cfg.Key = MakeRateLimitKey(cfg.Type, r.RemoteAddr)
		fn, _ := lim()
		res, _ := fn()
		if res.Allowed < 1 {
			fmt.Printf("\n[MIDDLEWARE-%s-RATE-LIMITER] - [ACCESS DENIED] - Reason: %s, Retry After: %v\n", LimiterTypes[cfg.Type], http.StatusText(http.StatusTooManyRequests), res.RetryAfter)
			http.Error(rw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		} else {
			fmt.Printf("\n[MIDDLEWARE-%s-RATE-LIMITER] - [SUCCESS] - Allowed:%d, Remaining: %d\n", LimiterTypes[cfg.Type], res.Allowed, res.Remaining)
		}

		next.ServeHTTP(rw, r)
	})
}

// Creates a new rate limiter as a func, with input as limit tracking storage and config values
// Returns a func which checks allowed rate limiting based on key provided
func NewRedisLimiter(rc *redis.Client, cfg *RedisLimiterConfig) (CheckLimit, error) {
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
		rate = redis_rate.PerHour(loadVal("Authenticate"))
	case Otp:
		rate = redis_rate.PerMinute(loadVal("Otp"))
	case Get:
		rate = redis_rate.PerHour(loadVal("Get"))
	case Post:
		rate = redis_rate.PerMinute(loadVal("Post"))
	}
	return rate
}

func loadVal(vk string) int {
	rate := strings.Split(os.Getenv(vk), ":")[1]
	val, err := strconv.Atoi(rate)
	if err != nil {
		return val
	}
	return DefaultLimits[vk]
}

// Make a unique rate limiting key by using limiter type and identifier key
func MakeRateLimitKey(ltype LimiterType, key string) string {
	return fmt.Sprintf("%s~%s", LimiterTypes[ltype], key)
}

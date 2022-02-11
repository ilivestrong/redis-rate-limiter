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

func NewRedisLimiterAsMW(rc *redis.Client, cfg *RedisLimiterConfig, next http.Handler) http.Handler {
	lim := func() (CheckLimit, error) {
		fmt.Printf("MIDDLEWARE CFG.Key: %s", cfg.Key)
		return NewRedisLimiter(rc, cfg)
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		cfg.Key = MakeRateLimitKey(cfg.Type, r.RemoteAddr)
		fn, _ := lim()
		res, _ := fn()
		if res.Allowed < 1 {
			fmt.Println("[MIDDLEWARE-RATE-LIMITER] - ACCESS DENIED")
			http.Error(rw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(rw, r)
	})
}

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
		rate = redis_rate.PerHour(loadVal("Authenticate")) // each client can send 10 authentication requests per hour
	case Otp:
		rate = redis_rate.PerMinute(loadVal("Otp")) // each client can send 5 otp/resend requests per minute
	case Get:
		rate = redis_rate.PerHour(loadVal("Get")) // each client can send 100 data requests per hour
	case Post:
		rate = redis_rate.PerMinute(loadVal("Post")) // each client can send 2 post requests per minute
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

func MakeRateLimitKey(ltype LimiterType, key string) string {
	return fmt.Sprintf("%s~%s", LimiterTypes[ltype], key)
}

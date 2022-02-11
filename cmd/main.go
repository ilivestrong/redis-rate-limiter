package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-redis/redis_rate/v9"
	"github.com/google/uuid"
	"github.com/ilivestrong/rate-limit-poc/internal"
	"github.com/ilivestrong/rate-limit-poc/internal/limiters"
	"github.com/joho/godotenv"
)

type AuthRequest struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

var (
	rc                   internal.RedisClient
	key                  = "localhost"
	ErrRedisGetExpired   = errors.New("redis server didn't respond in time")
	ErrRedisWriteExpired = errors.New("redis server didn't write in time")
)

func main() {
	envErr := godotenv.Load("config.env")
	if envErr != nil {
		fmt.Println("failed to load env, default settings will be used")
	}

	rc = internal.RedisClient{
		Client: internal.New(),
	}
	defer rc.Close()

	mux := http.NewServeMux()

	// handler with rate limited used as non-middleware
	mux.HandleFunc("/otp", otpHandler)

	// middleware based rate limiter for POST /auth endpoint
	authMW := limiters.NewRedisLimiterAsMW(rc.Client, &limiters.RedisLimiterConfig{
		Ctx:  context.Background(),
		Type: limiters.Authenticate,
	}, http.HandlerFunc(authHandler))
	mux.Handle("/auth", authMW)

	// handler without any rate limiting
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
	if r.Method == http.MethodPost {
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
		fmt.Printf("RECEIVED AUTHENTICATE REQUEST for Type: %s, Value: %s\n", authReq.Type, authReq.Value)

		setCookie(w) // Creates a new session_id cookie to be sent in response headers
		w.Write([]byte("Authentication successful!"))
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func otpHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// rate limiting
	lim := getLimiter(limiters.Otp, limiters.MakeRateLimitKey(limiters.Otp, r.RemoteAddr))
	res, _ := lim()
	fmt.Println(getMessage("Otp", res))
	if res.Allowed < 1 {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}

	// check session validity
	s, err := getSessionID(r)
	if err != nil || s == "" {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// create/reuse existing otp
	var otp string
	oldOtp, err := rc.Get(ctx, s)
	if err != nil {
		fmt.Printf("No existing Otp exists for : %s, generating new one.....\n", s)
	}

	if oldOtp != "" {
		otp = oldOtp.(string) // reuse old otp
		fmt.Printf("Non-expired existing Otp - %s exists, will reuse.....\n", otp)
	} else {
		otp = fmt.Sprintf("%d", rand.Intn(999999)) // generate new fake otp
		rc.Store(ctx, s, otp)                      // put new otp to redis for current user/session_id
	}

	select {
	case <-ctx.Done():
		fmt.Println(ErrRedisWriteExpired)
	default:
		fmt.Fprintf(w, "Here is your new OTP: %s", otp)
	}
}

func getMessage(l string, res *redis_rate.Result) string {
	if res.Allowed < 1 {
		return fmt.Sprintf("[%s-Rate-limiter]- [ACCESS DENIED] - Key: %s, Reason: %s, Retry After:%v\n", l, res.Limit.String(), http.StatusText(http.StatusTooManyRequests), res.RetryAfter)
	} else {
		return fmt.Sprintf("[%s-Rate-limiter]- [SUCCESS]- Key: %s, Allowed: %d, Remaning: %d\n", l, key, res.Allowed, res.Remaining)
	}
}

func getLimiter(lt limiters.LimiterType, key string) func() (*redis_rate.Result, error) {
	newLimiter, err := limiters.NewRedisLimiter(rc.Client, &limiters.RedisLimiterConfig{
		Ctx:  context.Background(),
		Key:  key,
		Type: lt,
	})
	if err != nil {
		panic(err)
	}
	return newLimiter
}

func getSessionID(r *http.Request) (string, error) {
	c, err := r.Cookie("session_id")
	if err != nil {
		return "", errors.New("no session_id cookie received")
	}
	if time.Now().Before(c.Expires) {
		return "", errors.New("your session_id has expired or invalid")
	}
	return c.Value, nil
}

func setCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   "session_id",
		Value:  uuid.NewString(),
		MaxAge: 60,
	}
	fmt.Println("New Session_id: ", cookie.Value)
	http.SetCookie(w, cookie)
}

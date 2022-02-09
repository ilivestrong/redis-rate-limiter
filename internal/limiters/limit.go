package limiters

import (
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(1, 3)

func Limit(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			fmt.Println(http.StatusText(http.StatusTooManyRequests))
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		}
		fmt.Printf("%v", r.RemoteAddr)
		next.ServeHTTP(w, r)

	})
}

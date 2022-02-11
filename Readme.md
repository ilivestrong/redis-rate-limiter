# RATE_LIMIT_POC

## Notes

- This POC is based on `github.com/go-redis/redis_rate/v9` package for rate limiting, which internally implements Leaky Bucket rate algorithm.

- This package relies on a Redis instance for persisting rate limiting specific information. This POC assumes that there is a Redis instance running on your machine at `PORT=6379`.

- This POC demonstrates both middleware based and standalone function based implementations for rate limiting.

- There are 3 endpoints being served from the server:  

## Endpoint 1

* `POST /auth` - This POST endpoint is a simulation of an authentication endpoint where user will provide their `Identification Type(passport/nationalid/taxid)` and the `value` of that identification in the request body as *JSON*.  

        Success response also writes a cookie on the client named - `session_id` which has a `MaxAge` of 60 secs.

`Request`  

```
    {
        "type": "passport",
        "value": "XXXXXXX"
    }
```

`Response 1 - Success`
 ```js
Authentication successful
```

`Response 1 - Exceeded Rate Limit quota`
 ```js
Too Many Requests
```


---
## Endpoint 2


 * `GET /otp` - This GET endpoint is a simulation of when we identified customer in `Nana` database and now we need to send an `OTP` for them to validate.  
 *This API requires `session_id` cookie to be present on client, hence `/auth` endpoint needs to be invoked first*  


`Response 1 - Success 200 OK`
 ```js
Here is your new OTP: <new/existing otp>
```

 `Response 2 - 400 Bad Request`
 ```js
no session_id cookie received
```

`Response 3 - 429 Too many requests`
 ```js
Too Many Requests
```
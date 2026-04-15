package main

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

var secretKey = []byte("jndsifhvusdkhbfjdsfbgljdbgfvljdsgvjld")

type Claims struct {
	User string `json:"user"`
	jwt.RegisteredClaims
}

func tokenRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		tokenStr := r.Header.Get("token")
		if tokenStr == "" {
			http.Error(w, "token is missing", 403)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(
			tokenStr,
			claims,
			func(t *jwt.Token) (interface{}, error) {
				return secretKey, nil
			},
		)

		if err != nil || !token.Valid {
			http.Error(w, "token is invalid", 403)
			return
		}

		ctx := context.WithValue(r.Context(), "user", claims)
		next(w, r.WithContext(ctx))
	}
}

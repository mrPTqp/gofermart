// internal/middleware/auth_mw.go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/mrPTqp/gofermart/internal/contextkey"
	"github.com/mrPTqp/gofermart/internal/handler"
	"go.uber.org/zap"
)

func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	secret := []byte(jwtSecret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := contextkey.LoggerFromContext(r.Context()) // ⬅️ из контекста
			log.Debug("auth middleware: started")

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Debug("auth middleware: missing Authorization header")
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				log.Debug("auth middleware: invalid authorization format")
				http.Error(w, "invalid authorization format", http.StatusUnauthorized)
				return
			}

			tokenString := authHeader[len(bearerPrefix):]
			log.Debug("auth middleware: parsing JWT token")

			claims := &handler.UserClaims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return secret, nil
			})

			if err != nil || !token.Valid {
				log.Debug("auth middleware: invalid or expired token", zap.Error(err))
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			log.Info("auth middleware: user authenticated successfully",
				zap.Int64("userID", claims.UserID))

			ctx := context.WithValue(r.Context(), handler.UserIDKeyContext, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

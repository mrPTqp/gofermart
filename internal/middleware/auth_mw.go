// internal/middleware/auth_mw.go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/mrPTqp/gofermart/internal/handler"
	"go.uber.org/zap"
)

// AuthMiddleware возвращает middleware с проверкой JWT
// Принимает logger и jwtSecret извне
func AuthMiddleware(logger *zap.SugaredLogger, jwtSecret string) func(http.Handler) http.Handler {
	secret := []byte(jwtSecret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Debug("auth middleware: started")

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Debug("auth middleware: missing Authorization header")
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				logger.Debug("auth middleware: invalid authorization format")
				http.Error(w, "invalid authorization format", http.StatusUnauthorized)
				return
			}

			tokenString := authHeader[len(bearerPrefix):]
			logger.Debug("auth middleware: parsing JWT token")

			claims := &handler.UserClaims{}

			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				if token.Method.Alg() != "HS256" {
					return nil, fmt.Errorf("unexpected signing algorithm: %s", token.Method.Alg())
				}
				return secret, nil
			})

			if err != nil {
				logger.Debugf("auth middleware: JWT parse error: %v", err)
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			if !token.Valid {
				logger.Debug("auth middleware: token is not valid")
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			logger.Infof("auth middleware: user authenticated successfully, userID=%d", claims.UserID)

			ctx := context.WithValue(r.Context(), handler.UserIDKeyContext, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

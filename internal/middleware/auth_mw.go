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

const jwtSecretKey = "supersecretkey" // Должно быть в env переменных

// AuthMiddleware возвращает middleware с логированием
func AuthMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Debug("auth middleware: started")

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Debug("auth middleware: missing Authorization header")
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			logger.Debugf("auth middleware: Authorization header = %s", authHeader)

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				logger.Debug("auth middleware: invalid authorization format")
				http.Error(w, "invalid authorization format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			logger.Debug("auth middleware: parsing JWT token")

			claims := &handler.UserClaims{}

			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					logger.Warnf("auth middleware: unexpected signing method: %v", token.Header["alg"])
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(jwtSecretKey), nil
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

			// Успешная аутентификация
			logger.Infof("auth middleware: user authenticated successfully, userID=%d", claims.UserID)

			// Передача userID в контекст
			ctx := context.WithValue(r.Context(), handler.UserIDKeyContext, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

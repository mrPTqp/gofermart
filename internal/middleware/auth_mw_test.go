// internal/middleware/auth_mw_test.go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/mrPTqp/gofermart/internal/contextkey"
	"github.com/mrPTqp/gofermart/internal/handler"
	"go.uber.org/zap/zaptest"
)

func TestAuthMiddleware(t *testing.T) {
	jwtSecret := "test-secret-key"
	userID := int64(123)

	// Создаем middleware
	middleware := AuthMiddleware(jwtSecret)

	// Целевой обработчик (для успешного прохождения)
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(handler.UserIDKeyContext).(int64)
		if !ok {
			http.Error(w, "user ID not found in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf("user %d authenticated", userID)))
	})

	tests := []struct {
		name           string
		authHeader     string
		token          string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing Authorization header",
			authHeader:     "",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authorization header required",
		},
		{
			name:           "invalid auth prefix",
			authHeader:     "Basic abc123",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid authorization format",
		},
		{
			name:           "empty Bearer token",
			authHeader:     "Bearer ",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid or expired token",
		},
		{
			name:           "invalid token signature",
			authHeader:     "Bearer ",
			token:          generateToken(userID, "wrong-secret"),
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid or expired token",
		},
		{
			name:           "expired token",
			authHeader:     "Bearer ",
			token:          generateExpiredToken(userID, jwtSecret),
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid or expired token",
		},
		{
			name:           "valid token",
			authHeader:     "Bearer ",
			token:          generateToken(userID, jwtSecret),
			expectedStatus: http.StatusOK,
			expectedBody:   "user 123 authenticated",
		},
		{
			name:           "malformed token",
			authHeader:     "Bearer ",
			token:          "invalid.jwt.token",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid or expired token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем запрос
			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" && tt.token != "" {
				req.Header.Set("Authorization", tt.authHeader+tt.token)
			} else if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Настраиваем логгер и контекст
			logger := zaptest.NewLogger(t)
			ctx := context.WithValue(req.Context(), contextkey.LoggerKey, logger)
			req = req.WithContext(ctx)

			// Создаем ResponseRecorder
			rr := httptest.NewRecorder()

			// Запускаем middleware + обработчик
			handler := middleware(successHandler)
			handler.ServeHTTP(rr, req)

			// Проверяем статус
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			// Проверяем тело ответа
			body := strings.TrimSpace(rr.Body.String())
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("handler returned unexpected body: got %v want to contain %v",
					body, tt.expectedBody)
			}
		})
	}
}

// generateToken генерирует валидный JWT токен для тестов
func generateToken(userID int64, secret string) string {
	claims := &handler.UserClaims{
		UserID: userID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString([]byte(secret))
	return signedToken
}

// generateExpiredToken генерирует просроченный JWT токен
func generateExpiredToken(userID int64, secret string) string {
	claims := &handler.UserClaims{
		UserID: userID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(), // уже истек
			IssuedAt:  time.Now().Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString([]byte(secret))
	return signedToken
}

// internal/middleware/middleware.go
package middleware

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/mrPTqp/gofermart/internal/handler"
)

const jwtSecretKey = "supersecretkey" // Должно быть в env

// LoggingMiddleware — логирует полностью запрос и ответ: метод, путь, заголовки, тело, статус, время
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// --- Логирование запроса ---

		var reqBody string
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("ERROR reading request body: %v", err)
			} else {
				reqBody = string(bodyBytes)
			}
			r.Body = io.NopCloser(strings.NewReader(reqBody)) // Восстанавливаем тело
		}

		log.Printf("=== REQUEST ===\n"+
			"Addr: %s\n"+
			"Method: %s\n"+
			"Path: %s\n"+
			"Headers: %v\n"+
			"Body: %s",
			r.RemoteAddr, r.Method, r.URL.Path,
			r.Header, reqBody)

		// --- Подготовка обёртки для ответа ---

		ww := &loggingResponseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			body:           new(strings.Builder),
		}

		next.ServeHTTP(ww, r)

		// --- Логирование ответа ---

		// Собираем заголовки ответа в map для чистого вывода
		respHeaders := make(map[string][]string)
		for k, v := range ww.Header() {
			respHeaders[k] = v
		}

		log.Printf("=== RESPONSE ===\n"+
			"Status: %d\n"+
			"Duration: %v\n"+
			"Headers: %v\n"+
			"Body: %s",
			ww.status,
			time.Since(start),
			respHeaders,
			ww.body.String())
	})
}

// GzipMiddleware — поддержка сжатия gzip
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Распаковка запроса, если он сжат
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gz
		}

		// Сжатие ответа, если клиент поддерживает
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			w = gzipResponseWriter{Writer: gz, ResponseWriter: w}
		}

		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware — проверка JWT токена
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "invalid authorization format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		claims := &handler.UserClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecretKey), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Передача userID в контекст
		ctx := context.WithValue(r.Context(), handler.UserIDKeyContext, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingResponseWriter — перехватывает статус, тело и заголовки ответа
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	body   *strings.Builder
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// gzipResponseWriter — обёртка для сжатия ответа
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

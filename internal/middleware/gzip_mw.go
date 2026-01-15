// internal/middleware/gzip_mw.go
package middleware

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// GzipMiddleware возвращает chi-совместимое middleware, которое:
// - сжимает ответ, если клиент поддерживает gzip и Content-Type разрешён
// - распаковывает тело запроса, если оно прислано в gzip
func GzipMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// === 1. Поддержка сжатия в ответе (gzip writer) ===
			supportsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
			cw := newCompressWriter(w, supportsGzip, logger)
			defer cw.Close() // гарантированно закроем writer
			w = cw           // подменяем ResponseWriter

			// === 2. Разжатие входящего тела, если оно сжато ===
			if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
				cr, err := newCompressReader(r.Body)
				if err != nil {
					logger.Error("failed to create gzip reader", zap.Error(err))
					http.Error(w, "invalid gzip data", http.StatusBadRequest)
					return
				}
				defer cr.Close()
				r.Body = cr
			}

			// === 3. Передаём управление следующему обработчику ===
			next.ServeHTTP(w, r)
		})
	}
}

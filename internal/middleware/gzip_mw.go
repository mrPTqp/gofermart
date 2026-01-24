package middleware

import (
	"net/http"
	"strings"

	"github.com/mrPTqp/gofermart/internal/contextkey"
	"go.uber.org/zap"
)

func GzipMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := contextkey.LoggerFromContext(r.Context())

			supportsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
			cw := newCompressWriter(w, supportsGzip, log)
			defer cw.Close()
			w = cw

			if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
				cr, err := newCompressReader(r.Body)
				if err != nil {
					log.Error("failed to create gzip reader", zap.Error(err))
					http.Error(w, "invalid gzip data", http.StatusBadRequest)
					return
				}
				defer cr.Close()
				r.Body = cr
			}

			next.ServeHTTP(w, r)
		})
	}
}

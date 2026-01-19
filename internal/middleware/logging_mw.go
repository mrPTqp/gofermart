package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	bodyBuf *bytes.Buffer
	status  int
	size    int
}

func (lw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lw.ResponseWriter.Write(b)
	lw.size += n
	lw.bodyBuf.Write(b)
	return n, err
}

func (lw *loggingResponseWriter) WriteHeader(statusCode int) {
	lw.status = statusCode
	lw.ResponseWriter.WriteHeader(statusCode)
}

func readBody(r io.ReadCloser) ([]byte, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}
	r.Close()
	return buf.Bytes(), nil
}

// LoggingMiddleware использует *zap.SugaredLogger
func LoggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Читаем тело запроса
			var reqBody []byte
			if r.Body != nil {
				var err error
				reqBody, err = readBody(r.Body)
				if err != nil {
					logger.Errorw("failed to read request body", "error", err, "uri", r.RequestURI)
				}
				r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			// Перехватываем ответ
			bodyBuf := &bytes.Buffer{}
			lw := &loggingResponseWriter{
				ResponseWriter: w,
				bodyBuf:        bodyBuf,
				status:         0,
				size:           0,
			}

			next.ServeHTTP(lw, r)

			duration := time.Since(start)
			isError := lw.status >= 400

			// Базовые поля (всегда логируются)
			args := []any{
				"method", r.Method,
				"uri", r.RequestURI,
				"status", lw.status,
				"duration", duration.String(),
			}

			// Дополнительные поля логируются только при ошибках
			if isError {
				// Сохраняем заголовки запроса
				reqHeaders := make(map[string]string)
				for k, v := range r.Header {
					reqHeaders[k] = strings.Join(v, ", ")
				}

				// Заголовки ответа
				resHeaders := make(map[string]string)
				for k, v := range w.Header() {
					resHeaders[k] = strings.Join(v, ", ")
				}

				// Добавляем подробные данные
				args = append(args,
					"query", r.URL.RawQuery,
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
					"response_size", lw.size,
					"req_body", truncateString(string(reqBody), 4096),
					"res_body", truncateString(bodyBuf.String(), 4096),
					"req_headers", reqHeaders,
					"res_headers", resHeaders,
				)
			}

			if isError {
				logger.Errorw("HTTP request/response (error)", args...)
			} else {
				logger.Infow("HTTP request", args...)
			}
		})
	}
}

// truncateString обрезает строку, если она слишком длинная
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}

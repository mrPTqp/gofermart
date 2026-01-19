package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/mrPTqp/gofermart/internal/contextkey"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	bodyBuf *bytes.Buffer
	status  int
	size    int
	err     error
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

func LoggingMiddleware(baseLogger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := uuid.NewString()
			start := time.Now()

			// Создаём логгер с базовыми полями
			log := baseLogger.With(
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.String("rid", rid),
			)

			// Добавляем логгер в контекст
			ctx := contextkey.WithLogger(r.Context(), log)
			r = r.WithContext(ctx)

			var reqBody []byte
			if r.Body != nil {
				var err error
				reqBody, err = readBody(r.Body)
				if err != nil {
					log.Error("failed to read request body", zap.Error(err))
				}
				r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			bodyBuf := &bytes.Buffer{}
			lw := &loggingResponseWriter{
				ResponseWriter: w,
				bodyBuf:        bodyBuf,
				status:         0,
				size:           0,
				err:            nil,
			}

			defer func() {
				// Берём логгер из контекста (он мог быть обогащён в ходе обработки)
				log := contextkey.LoggerFromContext(ctx)

				log = log.With(zap.String("duration", time.Since(start).String()))

				if p := recover(); p != nil {
					var msg string
					switch v := p.(type) {
					case string:
						msg = v
					case error:
						msg = v.Error()
					default:
						msg = fmt.Sprintf("%v", v)
					}

					// 🔥 Подробное логирование при панике: body, headers
					log = log.With(
						zap.String("event", "panic"),
						zap.String("msg", msg),
						zap.Int("status", http.StatusInternalServerError),
						zap.ByteString("request_body", reqBody),
						zap.Any("request_headers", r.Header),
						zap.ByteString("response_body", bodyBuf.Bytes()),
					)
					log.Error("Panic recovered")

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				status := lw.status
				if status == 0 {
					status = 200
				}
				log = log.With(zap.Int("status", status))

				// 🟢 Успешный запрос — только базовая информация
				if status >= 200 && status < 400 {
					log.Info("Successful request", zap.Int("size", lw.size))
					return
				}

				// 🔴 Ошибка — логируем всё!
				log = log.With(
					zap.Int("size", lw.size),
					zap.ByteString("request_body", reqBody),
					zap.Any("request_headers", r.Header),
					zap.ByteString("response_body", bodyBuf.Bytes()),
				)
				if lw.err != nil {
					log.Error("Request failed", zap.Error(lw.err))
				} else {
					log.Warn("Client error", zap.Int("status", status))
				}
			}()

			next.ServeHTTP(lw, r)
		})
	}
}

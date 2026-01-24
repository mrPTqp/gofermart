package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type compressWriter struct {
	w              http.ResponseWriter
	zw             *gzip.Writer
	supportsGzip   bool
	logger         *zap.Logger
	shouldCompress bool
	headerWritten  bool
}

func newCompressWriter(w http.ResponseWriter, supportsGzip bool, logger *zap.Logger) *compressWriter {
	return &compressWriter{
		w:              w,
		zw:             gzip.NewWriter(w),
		supportsGzip:   supportsGzip,
		logger:         logger,
		shouldCompress: false,
		headerWritten:  false,
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if !c.headerWritten {
		c.WriteHeader(http.StatusOK)
	}
	if c.shouldCompress {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if c.headerWritten {
		return
	}
	c.headerWritten = true

	contentType := c.w.Header().Get("Content-Type")

	c.shouldCompress = c.supportsGzip && statusCode < 300 && (strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "text/css") ||
		strings.Contains(contentType, "application/javascript"))

	if c.shouldCompress {
		c.w.Header().Set("Content-Encoding", "gzip")
		c.w.Header().Del("Content-Length")
	}
	c.w.WriteHeader(statusCode)
}

func (c *compressWriter) Close() error {
	if c.shouldCompress && c.headerWritten {
		if err := c.zw.Close(); err != nil {
			return err
		}
	}
	return nil
}

type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

func (c compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}

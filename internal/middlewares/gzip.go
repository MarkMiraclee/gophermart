package middlewares

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func Gzip(log *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer func() {
				if err := gz.Close(); err != nil {
					log.Errorf("failed to close gzip writer: %v", err)
				}
			}()

			w.Header().Set("Content-Encoding", "gzip")
			next.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gz}, r)
		})
	}
}

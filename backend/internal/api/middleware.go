package api

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
)

type gzipResponseWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (w gzipResponseWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w gzipResponseWriter) WriteString(data string) (int, error) {
	return w.writer.Write([]byte(data))
}

type timingResponseWriter struct {
	gin.ResponseWriter
	start time.Time
	set   bool
}

func (w *timingResponseWriter) setTimingHeader() {
	if w.set {
		return
	}
	elapsed := time.Since(w.start)
	w.Header().Set("Server-Timing", fmt.Sprintf("app;dur=%.1f", float64(elapsed.Microseconds())/1000))
	w.set = true
}

func (w *timingResponseWriter) WriteHeader(code int) {
	w.setTimingHeader()
	w.ResponseWriter.WriteHeader(code)
}

func (w *timingResponseWriter) Write(data []byte) (int, error) {
	w.setTimingHeader()
	return w.ResponseWriter.Write(data)
}

func (w *timingResponseWriter) WriteString(data string) (int, error) {
	w.setTimingHeader()
	return w.ResponseWriter.WriteString(data)
}

func GzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()
		c.Writer = gzipResponseWriter{ResponseWriter: c.Writer, writer: gz}
		c.Next()
	}
}

func PerformanceMiddleware(cfg appconfig.Config) gin.HandlerFunc {
	slowAfter := time.Duration(cfg.PerfSlowRequestMS) * time.Millisecond
	if slowAfter <= 0 {
		slowAfter = 500 * time.Millisecond
	}
	return func(c *gin.Context) {
		start := time.Now()
		timingWriter := &timingResponseWriter{ResponseWriter: c.Writer, start: start}
		c.Writer = timingWriter
		c.Next()
		elapsed := time.Since(start)
		timingWriter.setTimingHeader()
		if elapsed >= slowAfter && cfg.LogLevel != "error" {
			log.Printf("slow request method=%s path=%s status=%d duration_ms=%s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), strconv.FormatFloat(float64(elapsed.Microseconds())/1000, 'f', 1, 64))
		}
	}
}

func StaticCacheControl(value string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", value)
		c.Next()
	}
}

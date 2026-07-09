package trace

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// responseWriter wraps gin.ResponseWriter to capture response data.
type responseWriter struct {
	gin.ResponseWriter
	capture *captureBuffer
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.capture.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.capture.Write([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// Middleware returns a Gin middleware that traces LLM API requests.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		recorder := GetRecorder()
		if recorder == nil {
			c.Next()
			return
		}

		cfg := recorder.cfg
		path := c.Request.URL.Path

		// Skip non-LLM paths unless RecordAllPaths is enabled.
		if !cfg.RecordAllPaths && !isLLMPath(path) {
			c.Next()
			return
		}

		startedAt := time.Now().UTC()
		rec := &Record{
			TraceID:   newID(),
			StartedAt: startedAt.Format(time.RFC3339Nano),
			Method:    c.Request.Method,
			Path:      path,
			Query:     c.Request.URL.RawQuery,
		}

		// User ID: read from context after auth middleware has run.
		// We defer this until c.Next() completes, then extract.
		// Client info
		rec.ClientIP = clientIP(c.Request)
		rec.UserAgent = c.Request.Header.Get("User-Agent")

		// Headers
		if cfg.CaptureHeaders {
			rec.RequestHeaders = sanitizeHeaders(c.Request.Header)
		}

		// Capture request body
		rec.RequestContentType = c.Request.Header.Get("Content-Type")
		reqBuf := &captureBuffer{limit: cfg.MaxCaptureBytes}
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil {
			rec.RequestBytes = int64(len(bodyBytes))
			reqBuf.Write(bodyBytes)
			// Restore body for downstream handlers
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		rec.requestCapture = reqBuf

		// Wrap writer to capture response
		respBuf := &captureBuffer{limit: cfg.MaxCaptureBytes}
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			capture:        respBuf,
		}
		c.Writer = rw

		// Execute the handler chain
		c.Next()

		// Extract user ID and username from auth context (set by TokenAuth middleware)
		var userID, userName string
		if uid, exists := c.Get("id"); exists {
			switch v := uid.(type) {
			case int:
				userID = strconv.Itoa(v)
			case string:
				userID = v
			}
		}
		if uname, exists := c.Get("username"); exists {
			if s, ok := uname.(string); ok {
				userName = s
			}
		}
		// Build human-readable user identifier
		if userID != "" && userName != "" {
			rec.AuthHash = userID + ":" + userName
		} else {
			rec.AuthHash = userID
		}

		// Fill response info
		endedAt := time.Now().UTC()
		rec.EndedAt = endedAt.Format(time.RFC3339Nano)
		rec.DurationMs = endedAt.Sub(startedAt).Milliseconds()
		rec.Status = c.Writer.Status()
		rec.ResponseContentType = c.Writer.Header().Get("Content-Type")
		rec.responseCapture = respBuf
		rec.ResponseBytes = respBuf.Total()
		rec.ResponseTruncated = respBuf.truncated

		// Decode bodies
		rec.RequestBody = decodeBody(reqBuf.Bytes(), rec.RequestContentType, reqBuf.truncated)
		rec.RequestTruncated = reqBuf.truncated
		rec.ResponseBody = decodeBody(respBuf.Bytes(), rec.ResponseContentType, respBuf.truncated)

		// Collect provider metadata from response headers
		rec.ProviderMetadata = map[string]string{}
		collectProviderHeaders(c.Writer.Header(), rec.ProviderMetadata)
		if len(rec.ProviderMetadata) == 0 {
			rec.ProviderMetadata = nil
		}

		// Extract model / stream / session from body
		fillRequestSummary(rec)
		fillResponseSummary(rec)

		// Capture headers from response if configured
		if cfg.CaptureHeaders {
			rec.ResponseHeaders = sanitizeHeaders(c.Writer.Header())
		}

		// Error info
		if len(c.Errors) > 0 {
			rec.Error = c.Errors.String()
		}

		// Enqueue for async processing
		recorder.Enqueue(rec)

		// Log trace summary
		if rec.Status >= 400 || rec.Error != "" {
			log.Printf("[TRACE] %s %s → %d %dms model=%s [ERROR]", rec.Method, rec.Path, rec.Status, rec.DurationMs, rec.Model)
		}
	}
}

// collectProviderHeaders picks known provider response headers from the response.
func collectProviderHeaders(headers http.Header, out map[string]string) {
	if out == nil {
		return
	}
	for _, k := range []string{
		"X-Request-Id",
		"Openai-Request-Id",
		"X-Ratelimit-Limit-Requests",
		"X-Ratelimit-Remaining-Requests",
		"X-Ratelimit-Limit-Tokens",
		"X-Ratelimit-Remaining-Tokens",
	} {
		if v := headers.Get(k); v != "" {
			out[k] = v
		}
	}
}

package trace

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Record is a single traced request/response pair.
type Record struct {
	TraceID             string              `json:"trace_id"`
	StartedAt           string              `json:"started_at"`
	EndedAt             string              `json:"ended_at"`
	DurationMs          int64               `json:"duration_ms"`
	Method              string              `json:"method"`
	Path                string              `json:"path"`
	Query               string              `json:"query,omitempty"`
	ClientIP            string              `json:"client_ip,omitempty"`
	UserAgent           string              `json:"user_agent,omitempty"`
	AuthHash            string              `json:"auth_hash,omitempty"`
	RequestContentType  string              `json:"request_content_type,omitempty"`
	ResponseContentType string              `json:"response_content_type,omitempty"`
	RequestHeaders      map[string][]string `json:"request_headers,omitempty"`
	ResponseHeaders     map[string][]string `json:"response_headers,omitempty"`
	Status              int                 `json:"status"`
	Error               string              `json:"error,omitempty"`
	RequestBytes        int64               `json:"request_bytes"`
	ResponseBytes       int64               `json:"response_bytes"`
	RequestTruncated    bool                `json:"request_truncated"`
	ResponseTruncated   bool                `json:"response_truncated"`
	Model               string              `json:"model,omitempty"`
	Stream              *bool               `json:"stream,omitempty"`
	RequestBody         any                 `json:"request_body,omitempty"`
	ResponseBody        any                 `json:"response_body,omitempty"`
	Usage               map[string]any      `json:"usage,omitempty"`
	ProviderMetadata    map[string]string   `json:"provider_metadata,omitempty"`
	SessionID           string              `json:"session_id,omitempty"`
	// populated internally during capture
	requestCapture  *captureBuffer
	responseCapture *captureBuffer
}

// captureBuffer captures writes up to a limit.
type captureBuffer struct {
	limit     int64
	buf       bytes.Buffer
	total     int64
	truncated bool
}

func (c *captureBuffer) Write(p []byte) (int, error) {
	c.total += int64(len(p))
	remaining := c.limit - int64(c.buf.Len())
	if remaining > 0 {
		if int64(len(p)) > remaining {
			c.buf.Write(p[:remaining])
			c.truncated = true
		} else {
			c.buf.Write(p)
		}
	} else if len(p) > 0 {
		c.truncated = true
	}
	return len(p), nil
}

func (c *captureBuffer) Bytes() []byte { return c.buf.Bytes() }
func (c *captureBuffer) Total() int64  { return c.total }

// decodeBody decodes a captured body into a presentable value.
func decodeBody(data []byte, contentType string, truncated bool) any {
	if len(data) == 0 {
		return nil
	}
	data = maybeGunzip(data)
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") || looksJSON(data) {
		var v any
		if err := common.Unmarshal(data, &v); err == nil {
			if truncated {
				return map[string]any{"truncated": true, "json": v}
			}
			return v
		}
	}

	if mediaType == "text/event-stream" {
		stream := parseOpenAIStream(string(data))
		if truncated {
			stream["truncated"] = true
		}
		return stream
	}

	if strings.HasPrefix(mediaType, "text/") || mediaType == "" || strings.Contains(mediaType, "json") {
		text := string(data)
		if truncated {
			return map[string]any{"truncated": true, "text": text}
		}
		return text
	}

	return map[string]any{
		"encoding":  "base64",
		"truncated": truncated,
		"data":      base64.StdEncoding.EncodeToString(data),
	}
}

func maybeGunzip(data []byte) []byte {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return data
	}
	defer gr.Close()
	out, err := io.ReadAll(io.LimitReader(gr, DefaultMaxCaptureBytes))
	if err != nil {
		return data
	}
	return out
}

func looksJSON(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

// parseOpenAIStream parses SSE chunk text into a summary map.
func parseOpenAIStream(raw string) map[string]any {
	var builder strings.Builder
	usage := map[string]any{}
	events := 0

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		events++
		var chunk map[string]any
		if err := common.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if u, ok := chunk["usage"].(map[string]any); ok && len(u) > 0 {
			usage = u
		}
		appendStreamContent(&builder, chunk)
	}

	result := map[string]any{
		"type":        "text/event-stream",
		"event_count": events,
	}
	if builder.Len() > 0 {
		result["text"] = builder.String()
	}
	if len(usage) > 0 {
		result["usage"] = usage
	}
	return result
}

func appendStreamContent(builder *strings.Builder, chunk map[string]any) {
	choices, ok := chunk["choices"].([]any)
	if !ok || len(choices) == 0 {
		return
	}
	first, ok := choices[0].(map[string]any)
	if !ok {
		return
	}
	if delta, ok := first["delta"].(map[string]any); ok {
		if text, ok := delta["content"].(string); ok {
			builder.WriteString(text)
		}
		if toolCalls, ok := delta["tool_calls"].([]any); ok && len(toolCalls) > 0 {
			b, _ := common.Marshal(toolCalls)
			builder.WriteString(string(b))
		}
	}
	if text, ok := first["text"].(string); ok {
		builder.WriteString(text)
	}
}

// fillRequestSummary extracts model, stream, and session from the request body.
func fillRequestSummary(rec *Record) {
	body, ok := rec.RequestBody.(map[string]any)
	if !ok {
		return
	}
	if model, ok := body["model"].(string); ok {
		rec.Model = model
	}
	if sid, ok := body["session_id"].(string); ok {
		rec.SessionID = sid
	}
	if meta, ok := body["client_metadata"].(map[string]any); ok {
		if sid, ok := meta["session_id"].(string); ok {
			rec.SessionID = sid
		}
	}
	if rec.SessionID == "" {
		rec.SessionID = rec.AuthHash
	}
	if stream, ok := body["stream"].(bool); ok {
		rec.Stream = &stream
	}
}

func fillResponseSummary(rec *Record) {
	if body, ok := rec.ResponseBody.(map[string]any); ok {
		if usage, ok := body["usage"].(map[string]any); ok {
			rec.Usage = usage
		}
	}
}

func sanitizeHeaders(headers http.Header) map[string][]string {
	out := map[string][]string{}
	for k, vals := range headers {
		lower := strings.ToLower(k)
		if lower == "authorization" ||
			lower == "cookie" ||
			lower == "set-cookie" ||
			lower == "x-api-key" ||
			strings.Contains(lower, "token") ||
			strings.Contains(lower, "secret") ||
			strings.Contains(lower, "key") {
			out[k] = []string{"[redacted]"}
			continue
		}
		out[k] = append([]string(nil), vals...)
	}
	return out
}

func hashAuth(auth string, salt string) string {
	auth = strings.TrimSpace(auth)
	if auth == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + auth))
	return hex.EncodeToString(sum[:])
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isLLMPath returns true when the path matches LLM API patterns.
func isLLMPath(path string) bool {
	path = strings.ToLower(path)
	for _, marker := range []string{
		"/chat/completions",
		"/completions",
		"/responses",
		"/messages",
		"/embeddings",
		"/rerank",
		"/images/",
		"/audio/",
	} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

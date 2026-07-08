package trace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// fileSink writes trace records as NDJSON to daily-rotated files.
type fileSink struct {
	mu       sync.Mutex
	dir      string
	file     *os.File
	current  string
}

func newFileSink(dir string) (*fileSink, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("trace: create log dir %s: %w", dir, err)
	}
	return &fileSink{dir: dir}, nil
}

func (s *fileSink) Write(rec *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	date := time.Now().UTC().Format("20060102")
	fname := "trace-" + date + ".ndjson"
	if fname != s.current {
		if s.file != nil {
			s.file.Close()
		}
		f, err := os.OpenFile(filepath.Join(s.dir, fname), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("trace: open %s: %w", fname, err)
		}
		s.file = f
		s.current = fname
	}

	data, err := common.Marshal(rec)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = s.file.Write(data)
	return err
}

func (s *fileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// langfuseSink sends traces to a Langfuse instance.
type langfuseSink struct {
	host      string
	publicKey string
	secretKey string
	client    *http.Client
}

func newLangfuseSink(host, publicKey, secretKey string) *langfuseSink {
	return &langfuseSink{
		host:      strings.TrimRight(host, "/"),
		publicKey: publicKey,
		secretKey: secretKey,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *langfuseSink) Send(rec *Record, environment string) error {
	input := rec.RequestBody
	output := rec.ResponseBody
	metadata := map[string]any{
		"method":                rec.Method,
		"path":                  rec.Path,
		"query":                 rec.Query,
		"status":                rec.Status,
		"duration_ms":           rec.DurationMs,
		"client_ip":             rec.ClientIP,
		"auth_hash":             rec.AuthHash,
		"request_bytes":         rec.RequestBytes,
		"response_bytes":        rec.ResponseBytes,
		"request_truncated":     rec.RequestTruncated,
		"response_truncated":    rec.ResponseTruncated,
		"provider_metadata":     rec.ProviderMetadata,
		"request_content_type":  rec.RequestContentType,
		"response_content_type": rec.ResponseContentType,
	}
	if rec.Stream != nil {
		metadata["stream"] = *rec.Stream
	}

	level := "DEFAULT"
	statusMessage := ""
	if rec.Error != "" || rec.Status >= 400 {
		level = "ERROR"
		statusMessage = firstNonEmpty(rec.Error, http.StatusText(rec.Status))
	}

	traceBody := map[string]any{
		"id":          rec.TraceID,
		"name":        "llm-proxy " + rec.Method + " " + rec.Path,
		"timestamp":   rec.StartedAt,
		"userId":      rec.AuthHash,
		"input":       input,
		"output":      output,
		"metadata":    metadata,
		"environment": environment,
		"tags":        []string{"new-api-trace"},
	}
	if rec.SessionID != "" {
		traceBody["sessionId"] = rec.SessionID
	}

	body := map[string]any{
		"batch": []map[string]any{
			{
				"id":        newID(),
				"timestamp": rec.StartedAt,
				"type":      "trace-create",
				"body":      traceBody,
			},
			{
				"id":        newID(),
				"timestamp": rec.EndedAt,
				"type":      "generation-create",
				"body": map[string]any{
					"id":            rec.TraceID + "-generation",
					"traceId":       rec.TraceID,
					"name":          firstNonEmpty(rec.Model, rec.Path),
					"startTime":     rec.StartedAt,
					"endTime":       rec.EndedAt,
					"model":         rec.Model,
					"input":         input,
					"output":        output,
					"usage":         langfuseUsage(rec.Usage),
					"metadata":      metadata,
					"level":         level,
					"statusMessage": statusMessage,
					"environment":   environment,
				},
			},
		},
	}

	payload, err := common.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", s.host+"/api/public/ingestion", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(s.publicKey, s.secretKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("langfuse status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func langfuseUsage(usage map[string]any) map[string]any {
	if usage == nil {
		return nil
	}
	out := map[string]any{}
	if v, ok := numberFromMap(usage, "prompt_tokens", "input_tokens"); ok {
		out["input"] = v
	}
	if v, ok := numberFromMap(usage, "completion_tokens", "output_tokens"); ok {
		out["output"] = v
	}
	if v, ok := numberFromMap(usage, "total_tokens"); ok {
		out["total"] = v
	}
	return out
}

func numberFromMap(m map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return nil, false
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

package trace

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultMaxCaptureBytes int64 = 4 << 20 // 4MB

// Config holds all trace configuration, loaded from environment variables.
type Config struct {
	Enabled              bool
	FileEnabled          bool
	TraceLogDir          string
	MaxCaptureBytes      int64
	QueueSize            int
	Workers              int
	CaptureHeaders       bool
	RecordAllPaths       bool
	AuthHashSalt         string
	LangfuseEnabled      bool
	LangfuseHost         string
	LangfusePublicKey    string
	LangfuseSecretKey    string
	LangfuseEnvironment  string
}

// LoadConfig reads trace configuration from environment variables.
func LoadConfig() Config {
	cfg := Config{
		Enabled:             envBool("TRACE_ENABLED", true),
		FileEnabled:         envBool("TRACE_FILE_ENABLED", true),
		TraceLogDir:         envString("TRACE_LOG_DIR", "data/traces"),
		MaxCaptureBytes:     envInt64("TRACE_MAX_CAPTURE_BYTES", DefaultMaxCaptureBytes),
		QueueSize:           envInt("TRACE_QUEUE_SIZE", 256),
		Workers:             envInt("TRACE_WORKERS", 4),
		CaptureHeaders:      envBool("TRACE_CAPTURE_HEADERS", false),
		RecordAllPaths:      envBool("TRACE_RECORD_ALL_PATHS", false),
		AuthHashSalt:        envString("TRACE_AUTH_HASH_SALT", ""),
		LangfuseEnabled:     envBool("LANGFUSE_ENABLED", false),
		LangfuseHost:        envString("LANGFUSE_HOST", ""),
		LangfusePublicKey:   envString("LANGFUSE_PUBLIC_KEY", ""),
		LangfuseSecretKey:   envString("LANGFUSE_SECRET_KEY", ""),
		LangfuseEnvironment: envString("LANGFUSE_ENVIRONMENT", "production"),
	}
	return cfg
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return def
}

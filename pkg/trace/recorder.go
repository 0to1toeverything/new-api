package trace

import (
	"log"
	"sync"
	"time"
)

// Recorder manages async trace record processing.
type Recorder struct {
	cfg          Config
	fileSink     *fileSink
	langfuseSink *langfuseSink
	jobs         chan *Record
	wg           sync.WaitGroup
	startedAt    time.Time
}

// NewRecorder creates and starts a trace recorder.
func NewRecorder(cfg Config) (*Recorder, error) {
	r := &Recorder{
		cfg:       cfg,
		jobs:      make(chan *Record, cfg.QueueSize),
		startedAt: time.Now(),
	}

	if cfg.FileEnabled {
		fs, err := newFileSink(cfg.TraceLogDir)
		if err != nil {
			return nil, err
		}
		r.fileSink = fs
	}

	if cfg.LangfuseEnabled && cfg.LangfuseHost != "" && cfg.LangfusePublicKey != "" {
		r.langfuseSink = newLangfuseSink(cfg.LangfuseHost, cfg.LangfusePublicKey, cfg.LangfuseSecretKey)
	}

	for i := 0; i < cfg.Workers; i++ {
		r.wg.Add(1)
		go r.worker()
	}

	log.Printf("[TRACE] recorder started: workers=%d queue=%d file=%v langfuse=%v",
		cfg.Workers, cfg.QueueSize, cfg.FileEnabled, cfg.LangfuseEnabled)
	return r, nil
}

func (r *Recorder) Enqueue(rec *Record) {
	select {
	case r.jobs <- rec:
	default:
		// Queue full — drop the trace rather than block the request.
		log.Printf("[TRACE] queue full, dropping trace %s", rec.TraceID)
	}
}

func (r *Recorder) Shutdown() {
	close(r.jobs)
	r.wg.Wait()
	if r.fileSink != nil {
		r.fileSink.Close()
	}
	log.Printf("[TRACE] recorder shutdown complete")
}

func (r *Recorder) worker() {
	defer r.wg.Done()
	for rec := range r.jobs {
		// Write to local file
		if r.fileSink != nil {
			if err := r.fileSink.Write(rec); err != nil {
				log.Printf("[TRACE] file write error: %v", err)
			}
		}
		// Send to Langfuse
		if r.langfuseSink != nil {
			if err := r.langfuseSink.Send(rec, r.cfg.LangfuseEnvironment); err != nil {
				log.Printf("[TRACE] langfuse send error: %v", err)
			}
		}
	}
}

// Singleton recorder instance.
var globalRecorder *Recorder
var initRecorderOnce sync.Once

// InitRecorder initializes the global trace recorder. Safe to call multiple times.
func InitRecorder() {
	initRecorderOnce.Do(func() {
		cfg := LoadConfig()
		if !cfg.Enabled {
			log.Println("[TRACE] disabled")
			return
		}
		var err error
		globalRecorder, err = NewRecorder(cfg)
		if err != nil {
			log.Printf("[TRACE] init failed: %v", err)
		}
	})
}

// GetRecorder returns the global recorder, or nil if tracing is disabled.
func GetRecorder() *Recorder {
	return globalRecorder
}

// ShutdownRecorder shuts down the global recorder gracefully.
func ShutdownRecorder() {
	if globalRecorder != nil {
		globalRecorder.Shutdown()
	}
}

// Concurrency test for new-api gateway.
// Usage:
//   go run scripts/concurrency_test.go \
//     -c 10 -n 100
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type result struct {
	ok     bool
	ms     int64
	status int
}

func main() {
	var (
		baseURL    string
		apiKey     string
		model      string
		concurrent int
		total      int
		timeout    int
	)
	flag.StringVar(&baseURL, "url", "http://localhost:3000", "new-api gateway URL")
	flag.StringVar(&apiKey, "key", "sk-jeQdRcYxerF2hgAnE6kScyGgWlENK7dLoZhQxTaQwyX1kC2I", "API key")
	flag.StringVar(&model, "model", "test-gpt-concurrency", "model name to use in request")
	flag.IntVar(&concurrent, "c", 10, "concurrency level")
	flag.IntVar(&total, "n", 100, "total requests")
	flag.IntVar(&timeout, "timeout", 60, "request timeout in seconds")
	flag.Parse()



	fmt.Printf("🚀 并发测试: url=%s, c=%d, n=%d, model=%s\n\n", baseURL, concurrent, total, model)

	// Job channel
	jobs := make(chan int, total)
	for i := range total {
		jobs <- i
	}
	close(jobs)

	results := make(chan result, total)

	var wg sync.WaitGroup
	start := time.Now()
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	// Worker goroutines
	for range concurrent {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				results <- doRequest(client, baseURL, apiKey, model)
			}
		}()
	}

	// Close results when all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate
	var (
		mu                              sync.Mutex
		ok, fail                        int
		totalMs                         int64
		minMs, maxMs                    int64 = -1, 0
		statusCounts                    = map[int]int64{}
	)

	for r := range results {
		totalMs += r.ms
		if r.ok {
			ok++
		} else {
			fail++
		}

		mu.Lock()
		statusCounts[r.status]++
		mu.Unlock()

		if minMs == -1 || r.ms < minMs {
			minMs = r.ms
		}
		if r.ms > maxMs {
			maxMs = r.ms
		}
	}

	elapsed := time.Since(start)
	qps := float64(total) / elapsed.Seconds()
	avgMs := float64(totalMs) / float64(total)
	successRate := float64(ok) / float64(total) * 100

	fmt.Printf("⏱  耗时: %v\n", elapsed)
	fmt.Printf("📊 总计: %d | 成功: %d | 失败: %d | 成功率: %.1f%%\n", total, ok, fail, successRate)
	fmt.Printf("⚡  QPS: %.1f\n", qps)
	fmt.Printf("📈 延迟 (ms): min=%d, avg=%.1f, max=%d\n", minMs, avgMs, maxMs)
	fmt.Printf("\n📋 HTTP 状态码分布:\n")
	for code := 200; code <= 599; code++ {
		mu.Lock()
		c := statusCounts[code]
		mu.Unlock()
		if c > 0 {
			label := "✅"
			if code >= 400 {
				label = "❌"
			}
			fmt.Printf("  %s %d: %d\n", label, code, c)
		}
	}

	if fail > 0 {
		fmt.Println("\n⚠️  有失败请求，请检查服务端日志排查原因。")
	}
}

func doRequest(client *http.Client, baseURL, apiKey, model string) result {
	body := fmt.Sprintf(`{
		"model": %q,
		"messages": [{"role": "user", "content": "hello, respond with just the word 'ok'"}],
		"max_tokens": 16
	}`, model)
	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader([]byte(body)))
	if err != nil {
		return result{status: -1, ms: 0}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-"+apiKey)

	start := time.Now()
	resp, err := client.Do(req)
	ms := time.Since(start).Milliseconds()

	if err != nil {
		return result{ok: false, ms: ms, status: -1}
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)
	return result{ok: resp.StatusCode == 200, ms: ms, status: resp.StatusCode}
}

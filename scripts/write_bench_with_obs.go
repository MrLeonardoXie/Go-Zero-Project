package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type benchResult struct {
	Total        int64
	Success      int64
	HTTPFail     int64
	BizFail      int64
	RequestError int64
	LatenciesMs  []float64
	Elapsed      time.Duration
}

type promSnapshot struct {
	RPS          float64
	P99Ms        float64
	Non2xxRPS    float64
	GoRoutines   float64
	MemoryBytes  float64
	QueryMissing []string
}

type jaegerSnapshot struct {
	ServiceExists bool
	TraceCount1h  int
	P95TraceMs    float64
}

func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:8889/v1/article/publish", "write endpoint")
	token := flag.String("token", "", "jwt token for Authorization header")
	concurrency := flag.Int("concurrency", 50, "concurrent workers")
	total := flag.Int("requests", 5000, "total requests")
	timeout := flag.Duration("timeout", 3*time.Second, "http timeout per request")

	promURL := flag.String("prom-url", "http://127.0.0.1:9090", "prometheus base url")
	promService := flag.String("prom-service", "article-api", "prometheus service_name label")
	jaegerURL := flag.String("jaeger-url", "http://127.0.0.1:16686", "jaeger base url")
	jaegerService := flag.String("jaeger-service", "article.rpc", "jaeger service name")

	flag.Parse()

	if strings.TrimSpace(*token) == "" {
		fmt.Println("missing -token, example:")
		fmt.Println("go run ./scripts/write_bench_with_obs.go -token \"<your_jwt>\"")
		return
	}

	fmt.Println("[1/3] collecting pre-benchmark Prometheus and Jaeger snapshot...")
	preProm := collectPrometheus(*promURL, *promService)
	preJaeger := collectJaeger(*jaegerURL, *jaegerService)

	fmt.Println("[2/3] running write benchmark...")
	result := runWriteBenchmark(*endpoint, *token, *concurrency, *total, *timeout)

	fmt.Println("[3/3] collecting post-benchmark Prometheus and Jaeger snapshot...")
	postProm := collectPrometheus(*promURL, *promService)
	postJaeger := collectJaeger(*jaegerURL, *jaegerService)

	printReport(result, preProm, postProm, preJaeger, postJaeger)
}

func runWriteBenchmark(endpoint, token string, concurrency, total int, timeout time.Duration) *benchResult {
	client := &http.Client{Timeout: timeout}

	latencies := make([]float64, 0, total)
	var latMu sync.Mutex

	var sent int64
	var success int64
	var httpFail int64
	var bizFail int64
	var requestErr int64

	start := time.Now()
	var wg sync.WaitGroup

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				idx := int(atomic.AddInt64(&sent, 1))
				if idx > total {
					return
				}

				payload := map[string]string{
					"title":       fmt.Sprintf("load-title-%d-%d", workerID, idx),
					"content":     strings.Repeat("go-zero-load-test-content-", 4),
					"description": "load test write path",
					"cover":       "https://example.com/cover.jpg",
				}
				body, _ := json.Marshal(payload)

				req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				t0 := time.Now()
				resp, err := client.Do(req)
				lat := time.Since(t0).Seconds() * 1000

				latMu.Lock()
				latencies = append(latencies, lat)
				latMu.Unlock()

				if err != nil {
					atomic.AddInt64(&requestErr, 1)
					continue
				}

				respBody, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()

				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					atomic.AddInt64(&httpFail, 1)
					continue
				}

				if isBizError(respBody) {
					atomic.AddInt64(&bizFail, 1)
					continue
				}

				atomic.AddInt64(&success, 1)
			}
		}(worker)
	}

	wg.Wait()
	elapsed := time.Since(start)

	return &benchResult{
		Total:        int64(total),
		Success:      success,
		HTTPFail:     httpFail,
		BizFail:      bizFail,
		RequestError: requestErr,
		LatenciesMs:  latencies,
		Elapsed:      elapsed,
	}
}

func isBizError(body []byte) bool {
	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		return true
	}
	codeValue, ok := v["code"]
	if !ok {
		return false
	}

	switch c := codeValue.(type) {
	case float64:
		return int(c) != 0
	case int:
		return c != 0
	case string:
		return c != "0"
	default:
		return true
	}
}

func collectPrometheus(baseURL, service string) *promSnapshot {
	queries := map[string]string{
		"rps":        fmt.Sprintf("sum(rate(http_server_requests_duration_ms_count{service_name=\"%s\"}[1m]))", service),
		"p99":        fmt.Sprintf("histogram_quantile(0.99, sum by (le) (rate(http_server_requests_duration_ms_bucket{service_name=\"%s\"}[1m])))", service),
		"non2xx_rps": fmt.Sprintf("sum(rate(http_server_requests_code_total{service_name=\"%s\",code!~\"2..\"}[1m]))", service),
		"goroutines": fmt.Sprintf("sum(go_goroutines{service_name=\"%s\"})", service),
		"memory":     fmt.Sprintf("sum(process_resident_memory_bytes{service_name=\"%s\"})", service),
	}

	s := &promSnapshot{}
	for k, q := range queries {
		v, ok := queryPrometheus(baseURL, q)
		if !ok {
			s.QueryMissing = append(s.QueryMissing, k)
			continue
		}
		switch k {
		case "rps":
			s.RPS = v
		case "p99":
			s.P99Ms = v
		case "non2xx_rps":
			s.Non2xxRPS = v
		case "goroutines":
			s.GoRoutines = v
		case "memory":
			s.MemoryBytes = v
		}
	}

	return s
}

func queryPrometheus(baseURL, promQL string) (float64, bool) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/v1/query")
	if err != nil {
		return 0, false
	}
	q := u.Query()
	q.Set("query", promQL)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, false
	}

	var data struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Value []any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, false
	}
	if data.Status != "success" || len(data.Data.Result) == 0 || len(data.Data.Result[0].Value) < 2 {
		return 0, false
	}

	vs, ok := data.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, false
	}

	v, err := parseFloat(vs)
	if err != nil {
		return 0, false
	}
	return v, true
}

func collectJaeger(baseURL, service string) *jaegerSnapshot {
	s := &jaegerSnapshot{}

	servicesURL := strings.TrimRight(baseURL, "/") + "/api/services"
	resp, err := http.Get(servicesURL)
	if err != nil {
		return s
	}
	servicesBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var servicesData struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(servicesBody, &servicesData); err != nil {
		return s
	}

	for _, svc := range servicesData.Data {
		if svc == service {
			s.ServiceExists = true
			break
		}
	}
	if !s.ServiceExists {
		return s
	}

	tracesURL := strings.TrimRight(baseURL, "/") + "/api/traces?service=" + url.QueryEscape(service) + "&lookback=1h&limit=50"
	tracesResp, err := http.Get(tracesURL)
	if err != nil {
		return s
	}
	traceBody, _ := io.ReadAll(tracesResp.Body)
	_ = tracesResp.Body.Close()

	var tracesData struct {
		Data []struct {
			Spans []struct {
				Duration int64 `json:"duration"`
			} `json:"spans"`
		} `json:"data"`
	}
	if err := json.Unmarshal(traceBody, &tracesData); err != nil {
		return s
	}

	s.TraceCount1h = len(tracesData.Data)
	if s.TraceCount1h == 0 {
		return s
	}

	traceDurMs := make([]float64, 0, s.TraceCount1h)
	for _, t := range tracesData.Data {
		maxUs := int64(0)
		for _, sp := range t.Spans {
			if sp.Duration > maxUs {
				maxUs = sp.Duration
			}
		}
		traceDurMs = append(traceDurMs, float64(maxUs)/1000.0)
	}
	s.P95TraceMs = percentile(traceDurMs, 95)

	return s
}

func printReport(result *benchResult, preProm, postProm *promSnapshot, preJaeger, postJaeger *jaegerSnapshot) {
	sort.Float64s(result.LatenciesMs)
	rps := float64(result.Total) / result.Elapsed.Seconds()
	errRate := float64(result.HTTPFail+result.BizFail+result.RequestError) / float64(result.Total) * 100

	fmt.Println("\n==== Write Benchmark Result ====")
	fmt.Printf("total=%d success=%d http_fail=%d biz_fail=%d request_error=%d\n",
		result.Total, result.Success, result.HTTPFail, result.BizFail, result.RequestError)
	fmt.Printf("elapsed=%s throughput=%.2f req/s error_rate=%.2f%%\n", result.Elapsed.Truncate(time.Millisecond), rps, errRate)
	fmt.Printf("latency_ms p50=%.2f p90=%.2f p95=%.2f p99=%.2f max=%.2f\n",
		percentile(result.LatenciesMs, 50), percentile(result.LatenciesMs, 90), percentile(result.LatenciesMs, 95), percentile(result.LatenciesMs, 99), max(result.LatenciesMs))

	fmt.Println("\n==== Prometheus Snapshot (pre -> post) ====")
	fmt.Printf("rps: %.3f -> %.3f\n", preProm.RPS, postProm.RPS)
	fmt.Printf("p99_ms: %.3f -> %.3f\n", preProm.P99Ms, postProm.P99Ms)
	fmt.Printf("non2xx_rps: %.3f -> %.3f\n", preProm.Non2xxRPS, postProm.Non2xxRPS)
	fmt.Printf("go_goroutines: %.0f -> %.0f\n", preProm.GoRoutines, postProm.GoRoutines)
	fmt.Printf("memory_bytes: %.0f -> %.0f\n", preProm.MemoryBytes, postProm.MemoryBytes)
	if len(postProm.QueryMissing) > 0 {
		fmt.Printf("missing_prom_queries: %s\n", strings.Join(postProm.QueryMissing, ","))
	}

	fmt.Println("\n==== Jaeger Snapshot (pre -> post) ====")
	fmt.Printf("service_exists: %v -> %v\n", preJaeger.ServiceExists, postJaeger.ServiceExists)
	fmt.Printf("trace_count_1h: %d -> %d\n", preJaeger.TraceCount1h, postJaeger.TraceCount1h)
	fmt.Printf("p95_trace_ms: %.3f -> %.3f\n", preJaeger.P95TraceMs, postJaeger.P95TraceMs)
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 100 {
		return values[len(values)-1]
	}

	rank := p / 100 * float64(len(values)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return values[lo]
	}
	frac := rank - float64(lo)
	return values[lo] + (values[hi]-values[lo])*frac
}

func max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func parseFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

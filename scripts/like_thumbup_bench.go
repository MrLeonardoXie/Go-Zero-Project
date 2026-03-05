package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	likesvc "leonardo/application/like/rpc/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type result struct {
	Total       int64
	Success     int64
	Failed      int64
	Elapsed     time.Duration
	LatenciesMs []float64
	FirstError  string
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "like-rpc gRPC address")
	concurrency := flag.Int("concurrency", 100, "concurrent workers")
	total := flag.Int("requests", 10000, "total requests")
	bizID := flag.String("biz", "article", "biz id")
	objID := flag.Int64("obj", 1, "target object id")
	baseUser := flag.Int64("base-user", 1000000, "base user id for generated users")
	userPool := flag.Int("user-pool", 0, "reuse users from a fixed pool size; 0 means unique user per request")
	timeout := flag.Duration("timeout", 2*time.Second, "per-request timeout")
	flag.Parse()

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := likesvc.NewLikeClient(conn)
	res := run(client, *concurrency, *total, *bizID, *objID, *baseUser, *userPool, *timeout)
	printResult(res)
}

func run(client likesvc.LikeClient, concurrency, total int, bizID string, objID, baseUser int64, userPool int, timeout time.Duration) *result {
	latencies := make([]float64, 0, total)
	var latMu sync.Mutex

	var sent int64
	var success int64
	var failed int64
	var firstErr atomic.Value

	start := time.Now()
	var wg sync.WaitGroup

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				idx := int(atomic.AddInt64(&sent, 1))
				if idx > total {
					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				userID := baseUser + int64(idx)
				if userPool > 0 {
					userID = baseUser + int64(idx%userPool)
				}
				req := &likesvc.ThumbupRequest{
					BizId:    bizID,
					ObjId:    objID,
					UserId:   userID,
					LikeType: 0,
				}
				t0 := time.Now()
				_, err := client.Thumbup(ctx, req)
				lat := time.Since(t0).Seconds() * 1000
				cancel()

				latMu.Lock()
				latencies = append(latencies, lat)
				latMu.Unlock()

				if err != nil {
					atomic.AddInt64(&failed, 1)
					if firstErr.Load() == nil {
						firstErr.Store(err.Error())
					}
				} else {
					atomic.AddInt64(&success, 1)
				}
			}
		}()
	}

	wg.Wait()

	res := &result{
		Total:       int64(total),
		Success:     success,
		Failed:      failed,
		Elapsed:     time.Since(start),
		LatenciesMs: latencies,
	}
	if v := firstErr.Load(); v != nil {
		res.FirstError = v.(string)
	}
	return res
}

func printResult(r *result) {
	sort.Float64s(r.LatenciesMs)
	qps := float64(r.Total) / r.Elapsed.Seconds()
	errRate := 0.0
	if r.Total > 0 {
		errRate = float64(r.Failed) / float64(r.Total) * 100
	}

	fmt.Println("==== Like Thumbup gRPC Benchmark ====")
	fmt.Printf("total=%d success=%d failed=%d\n", r.Total, r.Success, r.Failed)
	if r.FirstError != "" {
		fmt.Printf("first_error=%s\n", r.FirstError)
	}
	fmt.Printf("elapsed=%s throughput=%.2f req/s error_rate=%.2f%%\n", r.Elapsed.Truncate(time.Millisecond), qps, errRate)
	fmt.Printf("latency_ms p50=%.2f p90=%.2f p95=%.2f p99=%.2f max=%.2f\n",
		pct(r.LatenciesMs, 50), pct(r.LatenciesMs, 90), pct(r.LatenciesMs, 95), pct(r.LatenciesMs, 99), max(r.LatenciesMs))
}

func pct(values []float64, p float64) float64 {
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

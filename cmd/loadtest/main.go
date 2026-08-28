// Command loadtest is a dependency-free load generator for SoundStage. It
// exercises the two hottest AI/interaction paths: the SSE AI chat stream and
// the danmaku ingest endpoint. Run it against a live server:
//
//	go run ./cmd/loadtest -base http://localhost:8080 -vus 50 -duration 30s
//
// It needs only the Go standard library, so it runs without k6 installed.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	base        string
	rooms       int
	vus         int
	duration    time.Duration
	danmakuRate int
	message     string
	danmakuText string
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.base, "base", "http://localhost:8080", "SoundStage HTTP base URL")
	flag.IntVar(&cfg.rooms, "rooms", 1, "number of rooms to create and share across VUs")
	flag.IntVar(&cfg.vus, "vus", 50, "concurrent SSE chat virtual users")
	flag.DurationVar(&cfg.duration, "duration", 30*time.Second, "test duration")
	flag.IntVar(&cfg.danmakuRate, "danmaku-rate", 200, "danmaku POSTs per second (aggregate)")
	flag.StringVar(&cfg.message, "message", "房间现在多少人？礼物榜第一是谁？", "SSE chat message")
	flag.StringVar(&cfg.danmakuText, "danmaku-text", "加油主播！", "danmaku text")
	flag.Parse()

	if err := run(cfg); err != nil {
		log.Fatalf("loadtest: %v", err)
	}
}

func run(cfg config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration)
	defer cancel()

	roomIDs, err := createRooms(cfg, cfg.rooms)
	if err != nil {
		return err
	}
	if len(roomIDs) == 0 {
		return fmt.Errorf("no rooms available; is the server up at %s?", cfg.base)
	}
	log.Printf("created %d room(s): %v", len(roomIDs), roomIDs)

	var (
		sseCount int64
		sseErr   int64
		danCount int64
		danErr   int64
	)
	var sseMu sync.Mutex
	var sseDurations []int64 // milliseconds

	start := time.Now()
	var wg sync.WaitGroup

	// SSE chat workers: each VU streams back-to-back until the deadline.
	for i := 0; i < cfg.vus; i++ {
		wg.Add(1)
		go func(vu int) {
			defer wg.Done()
			client := &http.Client{Timeout: 0} // SSE can run up to agent_timeout; no client timeout
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				rid := roomIDs[rand.Intn(len(roomIDs))]
				t0 := time.Now()
				if err := streamChat(ctx, client, cfg.base, rid, fmt.Sprintf("vu-%d", vu), cfg.message); err != nil {
					atomic.AddInt64(&sseErr, 1)
				} else {
					atomic.AddInt64(&sseCount, 1)
					sseMu.Lock()
					sseDurations = append(sseDurations, time.Since(t0).Milliseconds())
					sseMu.Unlock()
				}
			}
		}(i)
	}

	// Danmaku workers: a ticker drives the aggregate POST rate.
	if cfg.danmakuRate > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			tick := time.NewTicker(time.Second / time.Duration(cfg.danmakuRate))
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					rid := roomIDs[rand.Intn(len(roomIDs))]
					go func() {
						if err := postDanmaku(client, cfg.base, rid, cfg.danmakuText); err != nil {
							atomic.AddInt64(&danErr, 1)
						} else {
							atomic.AddInt64(&danCount, 1)
						}
					}()
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
	elapsed := time.Since(start)

	printReport(report{
		elapsed:      elapsed,
		sseCount:     atomic.LoadInt64(&sseCount),
		sseErr:       atomic.LoadInt64(&sseErr),
		sseDurations: sseDurations,
		danCount:     atomic.LoadInt64(&danCount),
		danErr:       atomic.LoadInt64(&danErr),
	})
	return nil
}

// streamChat opens the SSE chat endpoint and reads until the "done" event or
// the stream ends, returning an error if the connection fails.
func streamChat(ctx context.Context, client *http.Client, base, roomID, userID, message string) error {
	u := fmt.Sprintf("%s/rooms/%s/ai/chat?user_id=%s&message=%s",
		base, roomID, userID, url.QueryEscape(message))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if bytes.HasPrefix(scanner.Bytes(), []byte("event: done")) {
			return nil
		}
	}
	return scanner.Err()
}

func postDanmaku(client *http.Client, base, roomID, text string) error {
	payload, _ := json.Marshal(map[string]string{"user_id": fmt.Sprintf("vu-%d", rand.Int()), "text": text})
	resp, err := client.Post(base+"/api/v1/rooms/"+roomID+"/danmaku", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func createRooms(cfg config, n int) ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		payload, _ := json.Marshal(map[string]string{
			"anchor_id": fmt.Sprintf("loadtest-%d", i),
			"title":     fmt.Sprintf("loadtest room %d", i),
		})
		resp, err := client.Post(cfg.base+"/api/v1/rooms", "application/json", bytes.NewReader(payload))
		if err != nil {
			return ids, fmt.Errorf("create room: %w", err)
		}
		var out struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if out.ID != "" {
			ids = append(ids, out.ID)
		}
	}
	return ids, nil
}

type report struct {
	elapsed      time.Duration
	sseCount     int64
	sseErr       int64
	sseDurations []int64
	danCount     int64
	danErr       int64
}

func printReport(r report) {
	sseTotal := r.sseCount + r.sseErr
	danTotal := r.danCount + r.danErr
	secs := r.elapsed.Seconds()
	if secs <= 0 {
		secs = 1
	}

	fmt.Println("\n================ SoundStage Load Test ================")
	fmt.Printf("duration:        %s\n", r.elapsed.Round(time.Millisecond))
	fmt.Println("--- SSE AI chat ---")
	fmt.Printf("requests:        %d (errors %d, err%% %.2f)\n", sseTotal, r.sseErr, pct(r.sseErr, sseTotal))
	if len(r.sseDurations) > 0 {
		sort.Slice(r.sseDurations, func(i, j int) bool { return r.sseDurations[i] < r.sseDurations[j] })
		fmt.Printf("latency ms:      avg %.1f  p50 %d  p95 %d  p99 %d\n",
			avg(r.sseDurations), percentile(r.sseDurations, 50), percentile(r.sseDurations, 95), percentile(r.sseDurations, 99))
	}
	fmt.Printf("throughput:      %.1f req/s\n", float64(r.sseCount)/secs)
	fmt.Println("--- Danmaku ingest ---")
	fmt.Printf("requests:        %d (errors %d, err%% %.2f)\n", danTotal, r.danErr, pct(r.danErr, danTotal))
	fmt.Printf("throughput:      %.1f req/s\n", float64(r.danCount)/secs)
	fmt.Println("======================================================")
}

func avg(s []int64) float64 {
	var sum int64
	for _, v := range s {
		sum += v
	}
	return float64(sum) / float64(len(s))
}

func percentile(s []int64, p int) int64 {
	if len(s) == 0 {
		return 0
	}
	idx := (p * len(s)) / 100
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

func pct(n, total int64) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

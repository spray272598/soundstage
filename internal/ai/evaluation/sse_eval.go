package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// SSELatencyTestCase represents a test case for SSE latency evaluation.
type SSELatencyTestCase struct {
	Name        string `json:"name"`
	Message     string `json:"message"`
	RoomID      string `json:"room_id"`
	UserID      string `json:"user_id"`
	Description string `json:"description"`
}

// SSELatencyTestSuite is a collection of SSE latency test cases.
type SSELatencyTestSuite struct {
	Cases []SSELatencyTestCase `json:"cases"`
}

// SSELatencyMetrics holds evaluation metrics for SSE latency.
type SSELatencyMetrics struct {
	TotalRequests      int           `json:"total_requests"`
	FirstTokenLatency  LatencyStats  `json:"first_token_latency"`
	FullResponseLatency LatencyStats `json:"full_response_latency"`
	SuccessCount       int           `json:"success_count"`
	ErrorCount         int           `json:"error_count"`
}

// LatencyStats holds latency statistics.
type LatencyStats struct {
	Min   time.Duration `json:"min"`
	Max   time.Duration `json:"max"`
	Mean  time.Duration `json:"mean"`
	Median time.Duration `json:"median"`
	P50   time.Duration `json:"p50"`
	P95   time.Duration `json:"p95"`
	P99   time.Duration `json:"p99"`
}

// EvaluateSSELatency runs SSE end-to-end latency evaluation.
func EvaluateSSELatency(ctx context.Context, agent aidomain.Agent, suite SSELatencyTestSuite, runsPerCase int) SSELatencyMetrics {
	metrics := SSELatencyMetrics{}
	var firstTokenLatencies []time.Duration
	var fullResponseLatencies []time.Duration
	successCount := 0
	errorCount := 0

	for _, tc := range suite.Cases {
		for i := 0; i < runsPerCase; i++ {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			firstTokenTime := time.Time{}
			fullResponseTime := time.Time{}
			startTime := time.Now()

			_, err := agent.Run(ctx, tc.RoomID, tc.UserID, tc.Message, func(event aidomain.AgentEvent) {
				if event.Type == aidomain.EventTextDelta && firstTokenTime.IsZero() {
					firstTokenTime = time.Now()
				}
				if event.Type == aidomain.EventDone {
					fullResponseTime = time.Now()
				}
			})

			elapsed := time.Since(startTime)

			if err != nil {
				errorCount++
				continue
			}

			successCount++

			if !firstTokenTime.IsZero() {
				firstTokenLatencies = append(firstTokenLatencies, firstTokenTime.Sub(startTime))
			} else {
				// No streaming, use full response time
				firstTokenLatencies = append(firstTokenLatencies, elapsed)
			}

			if !fullResponseTime.IsZero() {
				fullResponseLatencies = append(fullResponseLatencies, fullResponseTime.Sub(startTime))
			} else {
				fullResponseLatencies = append(fullResponseLatencies, elapsed)
			}
		}
	}

	metrics.TotalRequests = len(suite.Cases) * runsPerCase
	metrics.SuccessCount = successCount
	metrics.ErrorCount = errorCount
	metrics.FirstTokenLatency = calcLatencyStats(firstTokenLatencies)
	metrics.FullResponseLatency = calcLatencyStats(fullResponseLatencies)

	return metrics
}

func calcLatencyStats(latencies []time.Duration) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}

	stats := LatencyStats{
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Mean:   sum / time.Duration(len(sorted)),
		Median: sorted[len(sorted)/2],
		P50:    percentile(sorted, 0.50),
		P95:    percentile(sorted, 0.95),
		P99:    percentile(sorted, 0.99),
	}
	return stats
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// LoadSSELatencyTestSuite loads test cases from a JSON file.
func LoadSSELatencyTestSuite(path string) (SSELatencyTestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SSELatencyTestSuite{}, err
	}
	var suite SSELatencyTestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return SSELatencyTestSuite{}, err
	}
	return suite, nil
}

// SaveSSELatencyTestSuite saves test cases to a JSON file.
func SaveSSELatencyTestSuite(path string, suite SSELatencyTestSuite) error {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// PrintSSELatencyMetrics prints evaluation metrics.
func PrintSSELatencyMetrics(m SSELatencyMetrics) string {
	return fmt.Sprintf(`SSE End-to-End Latency Evaluation Results
=============================================
Total Requests: %d
Successful: %d
Errors: %d

First Token Latency:
  Min:    %v
  Mean:   %v
  Median: %v
  P50:    %v
  P95:    %v
  P99:    %v

Full Response Latency:
  Min:    %v
  Mean:   %v
  Median: %v
  P50:    %v
  P95:    %v
  P99:    %v
`,
		m.TotalRequests, m.SuccessCount, m.ErrorCount,
		m.FirstTokenLatency.Min, m.FirstTokenLatency.Mean, m.FirstTokenLatency.Median,
		m.FirstTokenLatency.P50, m.FirstTokenLatency.P95, m.FirstTokenLatency.P99,
		m.FullResponseLatency.Min, m.FullResponseLatency.Mean, m.FullResponseLatency.Median,
		m.FullResponseLatency.P50, m.FullResponseLatency.P95, m.FullResponseLatency.P99)
}

// TestSSELatencyEval is a test for SSE latency evaluation.
func TestSSELatencyEval(t *testing.T) {
	suitePath := filepath.Join("testdata", "sse_latency_eval_cases.json")
	suite, err := LoadSSELatencyTestSuite(suitePath)
	if err != nil {
		t.Fatalf("load test suite: %v", err)
	}

	t.Logf("Loaded %d test cases", len(suite.Cases))
	t.Log("Run with EVAL_ENABLED=1 to execute against real agent")
}
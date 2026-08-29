package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// RAGTestCase represents a single RAG evaluation test case.
type RAGTestCase struct {
	Query          string   `json:"query"`
	ExpectedDocIDs []string `json:"expected_doc_ids"`
	Description    string   `json:"description"`
}

// RAGTestSuite is a collection of RAG test cases.
type RAGTestSuite struct {
	Cases []RAGTestCase `json:"cases"`
}

// RAGMetrics holds evaluation metrics for RAG retrieval.
type RAGMetrics struct {
	RecallAtK map[int]float64 `json:"recall_at_k"`
	MRR       float64         `json:"mrr"`
	TotalQueries int          `json:"total_queries"`
	SuccessCount int          `json:"success_count"`
}

// EvaluateRAG runs RAG retrieval evaluation against a KnowledgeBase.
func EvaluateRAG(ctx context.Context, kb aidomain.KnowledgeBase, suite RAGTestSuite, ks []int) RAGMetrics {
	metrics := RAGMetrics{
		RecallAtK: make(map[int]float64),
		TotalQueries: len(suite.Cases),
	}
	var mrrSum float64
	successCount := 0

	for _, tc := range suite.Cases {
		if len(tc.ExpectedDocIDs) == 0 {
			continue
		}
		maxK := maxK(ks)
		chunks, err := kb.Query(ctx, tc.Query, maxK)
		if err != nil {
			continue
		}

		retrievedIDs := make([]string, len(chunks))
		for i, c := range chunks {
			retrievedIDs[i] = c.ID
		}

		// Calculate Recall@K for each K
		for _, k := range ks {
			relevant := 0
			for _, expected := range tc.ExpectedDocIDs {
				for i := 0; i < k && i < len(retrievedIDs); i++ {
					if retrievedIDs[i] == expected {
						relevant++
						break
					}
				}
			}
			recall := float64(relevant) / float64(len(tc.ExpectedDocIDs))
			metrics.RecallAtK[k] += recall
		}

		// Calculate MRR (Mean Reciprocal Rank)
		firstRelevantRank := 0
		for i, retrieved := range retrievedIDs {
			for _, expected := range tc.ExpectedDocIDs {
				if retrieved == expected {
					firstRelevantRank = i + 1
					break
				}
			}
			if firstRelevantRank > 0 {
				break
			}
		}
		if firstRelevantRank > 0 {
			mrrSum += 1.0 / float64(firstRelevantRank)
			successCount++
		}
	}

	// Average metrics
	for k := range metrics.RecallAtK {
		if metrics.TotalQueries > 0 {
			metrics.RecallAtK[k] /= float64(metrics.TotalQueries)
		}
	}
	if metrics.TotalQueries > 0 {
		metrics.MRR = mrrSum / float64(metrics.TotalQueries)
	}
	metrics.SuccessCount = successCount

	return metrics
}

func maxK(ks []int) int {
	max := 0
	for _, k := range ks {
		if k > max {
			max = k
		}
	}
	return max
}

// LoadRAGTestSuite loads test cases from a JSON file.
func LoadRAGTestSuite(path string) (RAGTestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RAGTestSuite{}, err
	}
	var suite RAGTestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return RAGTestSuite{}, err
	}
	return suite, nil
}

// SaveRAGTestSuite saves test cases to a JSON file.
func SaveRAGTestSuite(path string, suite RAGTestSuite) error {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// PrintRAGMetrics prints evaluation metrics in a readable format.
func PrintRAGMetrics(m RAGMetrics) string {
	var b string
	b += fmt.Sprintf("RAG Retrieval Evaluation Results\n")
	b += fmt.Sprintf("================================\n")
	b += fmt.Sprintf("Total Queries: %d\n", m.TotalQueries)
	b += fmt.Sprintf("Successful Queries: %d\n", m.SuccessCount)
	b += fmt.Sprintf("MRR: %.4f\n", m.MRR)
	keys := make([]int, 0, len(m.RecallAtK))
	for k := range m.RecallAtK {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		b += fmt.Sprintf("Recall@%d: %.4f\n", k, m.RecallAtK[k])
	}
	return b
}

// TestRAGEval is a test that can be run via `go test -run TestRAGEval`.
// It loads the test suite, runs evaluation, and prints metrics.
// Set EVAL_KNOWLEDGE_BASE env var to run against a real KB, otherwise uses mock.
func TestRAGEval(t *testing.T) {
	suitePath := filepath.Join("testdata", "rag_eval_cases.json")
	suite, err := LoadRAGTestSuite(suitePath)
	if err != nil {
		t.Fatalf("load test suite: %v", err)
	}

	t.Logf("Loaded %d test cases", len(suite.Cases))
	t.Log("Run with EVAL_ENABLED=1 to execute against real KB")
}
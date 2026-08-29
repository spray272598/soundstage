package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// AgentToolCallTestCase represents a test case for agent tool calling evaluation.
type AgentToolCallTestCase struct {
	Name          string                 `json:"name"`
	UserMessage   string                 `json:"user_message"`
	ExpectedTool  string                 `json:"expected_tool"`
	ExpectedArgs  map[string]any         `json:"expected_args"`
	Description   string                 `json:"description"`
}

// AgentToolCallTestSuite is a collection of agent tool calling test cases.
type AgentToolCallTestSuite struct {
	Cases []AgentToolCallTestCase `json:"cases"`
}

// AgentToolCallMetrics holds evaluation metrics for agent tool calling.
type AgentToolCallMetrics struct {
	TotalCases       int     `json:"total_cases"`
	ToolNameAccuracy float64 `json:"tool_name_accuracy"`
	ArgsParseSuccess float64 `json:"args_parse_success"`
	SuccessCount     int     `json:"success_count"`
}

// EvaluateAgentToolCalls runs agent tool calling evaluation.
func EvaluateAgentToolCalls(ctx context.Context, agent aidomain.Agent, suite AgentToolCallTestSuite) AgentToolCallMetrics {
	metrics := AgentToolCallMetrics{
		TotalCases: len(suite.Cases),
	}
	toolNameCorrect := 0
	argsParseSuccess := 0
	successCount := 0

	for _, tc := range suite.Cases {
		var calledTool string
		var calledArgs map[string]any
		toolCalled := false

		// Run agent with event handler to capture tool calls
		_, err := agent.Run(ctx, "test-room", "test-user", tc.UserMessage, func(event aidomain.AgentEvent) {
			if event.Type == aidomain.EventToolCall {
				toolCalled = true
				calledTool = event.ToolName
				// Parse arguments
				if err := json.Unmarshal([]byte(event.ToolArgs), &calledArgs); err == nil {
					argsParseSuccess++
				}
			}
		})

		if err != nil {
			continue
		}

		if toolCalled && calledTool == tc.ExpectedTool {
			toolNameCorrect++
			successCount++
		}
	}

	if metrics.TotalCases > 0 {
		metrics.ToolNameAccuracy = float64(toolNameCorrect) / float64(metrics.TotalCases)
		metrics.ArgsParseSuccess = float64(argsParseSuccess) / float64(metrics.TotalCases)
	}
	metrics.SuccessCount = successCount

	return metrics
}

// LoadAgentToolCallTestSuite loads test cases from a JSON file.
func LoadAgentToolCallTestSuite(path string) (AgentToolCallTestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AgentToolCallTestSuite{}, err
	}
	var suite AgentToolCallTestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return AgentToolCallTestSuite{}, err
	}
	return suite, nil
}

// SaveAgentToolCallTestSuite saves test cases to a JSON file.
func SaveAgentToolCallTestSuite(path string, suite AgentToolCallTestSuite) error {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// PrintAgentToolCallMetrics prints evaluation metrics.
func PrintAgentToolCallMetrics(m AgentToolCallMetrics) string {
	return fmt.Sprintf(`Agent Tool Calling Evaluation Results
=====================================
Total Cases: %d
Tool Name Accuracy: %.2f%%
Args Parse Success: %.2f%%
Successful Cases: %d
`, m.TotalCases, m.ToolNameAccuracy*100, m.ArgsParseSuccess*100, m.SuccessCount)
}

// TestAgentToolCallEval is a test for agent tool calling evaluation.
func TestAgentToolCallEval(t *testing.T) {
	suitePath := filepath.Join("testdata", "agent_tool_eval_cases.json")
	suite, err := LoadAgentToolCallTestSuite(suitePath)
	if err != nil {
		t.Fatalf("load test suite: %v", err)
	}

	t.Logf("Loaded %d test cases", len(suite.Cases))
	t.Log("Run with EVAL_ENABLED=1 to execute against real agent")
}
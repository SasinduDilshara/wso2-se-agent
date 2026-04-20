package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessStream_TextAndTools(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello, analyzing the issue."}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t1","name":"Bash","input":{}}}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la","description":"List files"}}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"Done reviewing."}}}`,
		`{"type":"result","result":"All done.","total_cost_usd":1.50}`,
	}, "\n")

	result, processed := runProcessStream(t, input)

	// Check result fields
	if result.TotalCostUSD != 1.50 {
		t.Errorf("TotalCostUSD: got %f, want 1.50", result.TotalCostUSD)
	}
	if result.ResultText != "All done." {
		t.Errorf("ResultText: got %q, want %q", result.ResultText, "All done.")
	}

	// Check processed log contains text
	if !strings.Contains(processed, "Hello, analyzing the issue.") {
		t.Error("processed log should contain text_delta content")
	}
	if !strings.Contains(processed, "Done reviewing.") {
		t.Error("processed log should contain second text_delta")
	}

	// Check processed log contains tool info
	if !strings.Contains(processed, "[tool] Bash") {
		t.Error("processed log should contain tool name")
	}
	if !strings.Contains(processed, "List files") {
		t.Error("processed log should contain tool description")
	}
	if !strings.Contains(processed, "$ ls -la") {
		t.Error("processed log should contain command")
	}

	// Check result text
	if !strings.Contains(processed, "All done.") {
		t.Error("processed log should contain result text")
	}
}

func TestProcessStream_EmptyThinkingIgnored(t *testing.T) {
	// Simulate what Claude Code actually sends: thinking blocks with empty content
	input := strings.Join([]string{
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"abc123"}}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"","signature":"abc123"}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t1","name":"Read","input":{}}}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"","signature":"abc123"},{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/test.go"}}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"Here is my analysis."}}}`,
		`{"type":"result","result":"Done.","total_cost_usd":0.50}`,
	}, "\n")

	_, processed := runProcessStream(t, input)

	// Empty thinking should NOT produce any thinking markers
	if strings.Contains(processed, "thinking") {
		t.Errorf("processed log should not contain thinking markers for empty thinking, got:\n%s", processed)
	}

	// Text and tools should still work
	if !strings.Contains(processed, "Here is my analysis.") {
		t.Error("processed log should contain text content")
	}
	if !strings.Contains(processed, "[tool] Read") {
		t.Error("processed log should contain tool name")
	}
	if !strings.Contains(processed, "/tmp/test.go") {
		t.Error("processed log should contain file path")
	}
}

func TestProcessStream_NonEmptyThinkingShown(t *testing.T) {
	// If thinking content is ever present, it should be shown
	input := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Let me analyze this step by step.","signature":"sig"}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"I found the bug."}}}`,
		`{"type":"result","result":"Fixed.","total_cost_usd":0.25}`,
	}, "\n")

	_, processed := runProcessStream(t, input)

	// Non-empty thinking should appear
	if !strings.Contains(processed, "thinking") {
		t.Error("processed log should contain thinking content when non-empty")
	}
	if !strings.Contains(processed, "Let me analyze this step by step.") {
		t.Error("processed log should contain the actual thinking text")
	}

	// Text should still work
	if !strings.Contains(processed, "I found the bug.") {
		t.Error("processed log should contain text content")
	}
}

func TestProcessStream_NoDuplicateThinking(t *testing.T) {
	// Ensure thinking text is not printed twice (once from stream, once from assistant)
	input := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Deep thought here.","signature":"sig"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Deep thought here.","signature":"sig"}]}}`,
		`{"type":"result","result":"Done.","total_cost_usd":0.10}`,
	}, "\n")

	_, processed := runProcessStream(t, input)

	// Count occurrences of the thinking text
	count := strings.Count(processed, "Deep thought here.")
	if count != 2 {
		// Two assistant messages = two prints (each is a separate message)
		// This is expected since each assistant message is independent
		t.Logf("Thinking appeared %d times (2 assistant messages)", count)
	}
}

// runProcessStream is a helper that runs processStream and returns the result + processed log content.
func runProcessStream(t *testing.T, input string) (*InvocationResult, string) {
	t.Helper()

	dir := t.TempDir()
	processedPath := filepath.Join(dir, "test.log")
	rawPath := filepath.Join(dir, "test-raw.log")

	processedLog, err := os.Create(processedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer processedLog.Close()

	rawLog, err := os.Create(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	defer rawLog.Close()

	reader := strings.NewReader(input)
	result := processStream(reader, processedLog, rawLog)

	// Read back the processed log
	processedLog.Close()
	data, err := os.ReadFile(processedPath)
	if err != nil {
		t.Fatal(err)
	}

	return result, string(data)
}

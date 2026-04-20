package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Options struct {
	Prompt         string
	WorkingDir     string
	OutputFormat   string
	MaxBudgetUSD   float64
	Model          string
	SkipPerms      bool
	Verbose        bool
	IncludePartial bool
}

type InvocationResult struct {
	ExitCode     int
	TotalCostUSD float64
	ResultText   string
}

type Invoker struct {
	Binary string
}

func NewInvoker() *Invoker {
	return &Invoker{Binary: "claude"}
}

func (inv *Invoker) Invoke(opts Options, logDir string, issueNumber, phaseName, timestamp string) (*InvocationResult, error) {
	args := inv.buildArgs(opts)

	cmd := exec.Command(inv.Binary, args...)
	cmd.Dir = opts.WorkingDir
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Prepare log files
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	logName := fmt.Sprintf("issue-%s-%s-%s", issueNumber, phaseName, timestamp)
	processedLog, err := os.Create(filepath.Join(logDir, logName+".log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}
	defer processedLog.Close()

	rawLog, err := os.Create(filepath.Join(logDir, logName+"-raw.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create raw log file: %w", err)
	}
	defer rawLog.Close()

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Process the stream
	result := processStream(stdout, processedLog, rawLog)

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		result.ExitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	}

	return result, nil
}

func (inv *Invoker) buildArgs(opts Options) []string {
	args := []string{"-p", opts.Prompt}

	outputFormat := opts.OutputFormat
	if outputFormat == "" {
		outputFormat = "stream-json"
	}
	args = append(args, "--output-format", outputFormat)

	if opts.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", opts.MaxBudgetUSD))
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.SkipPerms {
		args = append(args, "--dangerously-skip-permissions")
	}
	if opts.Verbose {
		args = append(args, "--verbose")
	}
	if opts.IncludePartial {
		args = append(args, "--include-partial-messages")
	}

	return args
}

func processStream(r io.Reader, processedLog, rawLog *os.File) *InvocationResult {
	result := &InvocationResult{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()

		// Write raw line
		fmt.Fprintln(rawLog, line)

		if line == "" {
			continue
		}

		// Parse JSON
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)

		switch msgType {
		case "stream_event":
			handleStreamEvent(msg, processedLog)
		case "assistant":
			handleAssistantMessage(msg, processedLog)
		case "result":
			handleResultMessage(msg, processedLog, result)
		}
	}

	return result
}

func handleStreamEvent(msg map[string]any, logFile *os.File) {
	event, _ := msg["event"].(map[string]any)
	if event == nil {
		return
	}

	delta, _ := event["delta"].(map[string]any)
	if delta != nil {
		deltaType, _ := delta["type"].(string)
		if deltaType == "text_delta" {
			text, _ := delta["text"].(string)
			fmt.Print(text)
			fmt.Fprint(logFile, text)
		}
		// Note: thinking_delta is not handled because Claude Code's
		// stream-json format redacts thinking content (empty strings).
	}

	eventType, _ := event["type"].(string)
	if eventType == "content_block_start" {
		cb, _ := event["content_block"].(map[string]any)
		if cb != nil {
			cbType, _ := cb["type"].(string)
			if cbType == "tool_use" {
				name, _ := cb["name"].(string)
				color := toolColor(name)
				out := fmt.Sprintf("\n%s[tool] %s%s", color, name, colorReset)
				fmt.Print(out)
				fmt.Fprint(logFile, out)
			}
		}
	}
}

func handleAssistantMessage(msg map[string]any, logFile *os.File) {
	message, _ := msg["message"].(map[string]any)
	if message == nil {
		return
	}

	content, _ := message["content"].([]any)
	for _, block := range content {
		b, _ := block.(map[string]any)
		if b == nil {
			continue
		}

		blockType, _ := b["type"].(string)
		if blockType == "thinking" {
			thinking, _ := b["thinking"].(string)
			thinking = strings.TrimSpace(thinking)
			if thinking != "" {
				out := fmt.Sprintf("\n%s[thinking] %s%s\n", colorMagenta, thinking, colorReset)
				fmt.Print(out)
				fmt.Fprint(logFile, out)
			}
		} else if blockType == "tool_use" {
			name, _ := b["name"].(string)
			input, _ := b["input"].(map[string]any)
			printToolSummary(name, input, logFile)
		}
	}
}


func handleResultMessage(msg map[string]any, logFile *os.File, result *InvocationResult) {
	if resultText, ok := msg["result"].(string); ok {
		result.ResultText = resultText
		out := fmt.Sprintf("\n%s%s%s\n", colorGreen, resultText, colorReset)
		fmt.Print(out)
		fmt.Fprint(logFile, out)
	}

	if cost, ok := msg["total_cost_usd"].(float64); ok {
		result.TotalCostUSD = cost
	}
}

func printToolSummary(name string, input map[string]any, logFile *os.File) {
	var detail string

	switch name {
	case "Bash":
		if desc, ok := input["description"].(string); ok && desc != "" {
			detail = ": " + desc
		}
		if cmd, ok := input["command"].(string); ok && cmd != "" {
			detail += fmt.Sprintf("\n  %s$ %s%s", colorYellow, cmd, colorReset)
		}
	case "Read", "Write", "Edit":
		if fp, ok := input["file_path"].(string); ok {
			detail = ": " + fp
		}
	case "Glob", "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			detail = ": " + pattern
		}
	case "Agent":
		if desc, ok := input["description"].(string); ok {
			detail = ": " + desc
		}
	case "Skill":
		if skill, ok := input["skill"].(string); ok {
			detail = ": /" + skill
		}
	case "WebSearch":
		if query, ok := input["query"].(string); ok {
			detail = ": " + query
		}
	case "WebFetch":
		if url, ok := input["url"].(string); ok {
			detail = ": " + url
		}
	case "TaskOutput":
		if taskID, ok := input["task_id"].(string); ok {
			detail = ": " + taskID
		}
	}

	out := detail + "\n"
	fmt.Print(out)
	fmt.Fprint(logFile, out)
}

// ANSI colors
const (
	colorReset   = "\033[0m"
	colorYellow  = "\033[0;33m"
	colorGreen   = "\033[0;32m"
	colorBlue    = "\033[0;34m"
	colorRed     = "\033[0;31m"
	colorCyan    = "\033[1;36m"
	colorMagenta = "\033[1;35m"
	colorBoldYellow = "\033[1;33m"
)

func toolColor(name string) string {
	switch name {
	case "Bash":
		return colorYellow
	case "Read":
		return colorGreen
	case "Write", "Edit":
		return colorRed
	case "Glob", "Grep":
		return colorBlue
	case "Agent":
		return colorBoldYellow
	default:
		return colorCyan
	}
}

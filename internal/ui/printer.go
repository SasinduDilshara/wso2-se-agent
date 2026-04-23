package ui

import (
	"fmt"
	"strings"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Red       = "\033[0;31m"
	Green     = "\033[0;32m"
	Yellow    = "\033[0;33m"
	Blue      = "\033[0;34m"
	Magenta   = "\033[0;35m"
	Cyan      = "\033[0;36m"
	White     = "\033[0;37m"
	BoldCyan  = "\033[1;36m"
	BoldGreen = "\033[1;32m"
	BoldRed   = "\033[1;31m"
)

type Printer struct {
	Verbose bool
}

func NewPrinter(verbose bool) *Printer {
	return &Printer{Verbose: verbose}
}

func (p *Printer) PhaseBanner(current, total int, name, kind string) {
	icon := "static"
	if kind == "ai" {
		icon = "claude"
	}
	fmt.Printf("\n%s[%d/%d] %-18s%s %s\n", Bold, current, total, name, Reset, icon)
}

func (p *Printer) PhaseSuccess(name, message string) {
	fmt.Printf("  %s\u2713%s  %s\n", Green, Reset, message)
}

func (p *Printer) PhaseFailed(name, message string) {
	fmt.Printf("  %s\u2717%s  %s\n", Red, Reset, message)
}

func (p *Printer) RiskGatePass(verdict string) {
	fmt.Printf("  %s\u25b8%s  pass  \u2014 verdict: %s, auto-proceeding\n", Green, Reset, verdict)
}

func (p *Printer) RiskGateBlocked(verdict, artifactPath string) {
	fmt.Printf("\n  %s\u25b8%s  %sBLOCKED%s  \u2014 verdict: %s\n", Red, Reset, BoldRed, Reset, verdict)
	fmt.Printf("    Review: %s\n", artifactPath)
	fmt.Printf("    Resume with: wso2-se-agent run ... --from fix\n")
}

func (p *Printer) Info(msg string) {
	fmt.Printf("  %s\n", msg)
}

func (p *Printer) Error(msg string) {
	fmt.Printf("%sError:%s %s\n", Red, Reset, msg)
}

func (p *Printer) Table(headers []string, rows [][]string) {
	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		fmt.Printf("  %-*s", widths[i]+2, h)
	}
	fmt.Println()

	// Print separator
	for _, w := range widths {
		fmt.Printf("  %s", strings.Repeat("-", w))
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Printf("  %-*s", widths[i]+2, cell)
			}
		}
		fmt.Println()
	}
}

func (p *Printer) PipelineSummary(product, version, issueURL string, phases []string) {
	fmt.Printf("\n%s=== WSO2 SE Agent ===%s\n", Bold, Reset)
	fmt.Printf("  Product: %s %s\n", product, version)
	fmt.Printf("  Issue:   %s\n", issueURL)
	fmt.Printf("  Phases:  %s\n\n", strings.Join(phases, " \u2192 "))
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/Tharsanan1/wso2-se-agent/internal/state"
	"github.com/Tharsanan1/wso2-se-agent/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status [workspace-path]",
	Short: "Show workspace state",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	wsPath := args[0]

	ws, err := state.Load(wsPath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if ws.IssueURL == "" {
		return fmt.Errorf("no state found at %s/.wse/state.json", wsPath)
	}

	fmt.Printf("Issue:   %s (#%s)\n", ws.IssueURL, ws.IssueNumber)
	fmt.Printf("Product: %s %s\n", ws.Product, ws.Version)
	fmt.Printf("Created: %s\n", ws.CreatedAt.Format("2006-01-02 15:04:05"))

	if ws.RiskScore != nil {
		fmt.Printf("Risk:    %d/10\n", *ws.RiskScore)
	}
	if ws.PRURL != "" {
		fmt.Printf("PR:      %s\n", ws.PRURL)
	}

	fmt.Println()

	printer := ui.NewPrinter(false)
	headers := []string{"Phase", "Status", "Cost", "Duration"}
	var rows [][]string

	for _, phaseName := range []string{"prereq", "workspace", "skills", "reproduce", "risk-assessment", "plan-and-fix", "verify", "test-coverage", "pr"} {
		if r, ok := ws.Phases[phaseName]; ok {
			cost := fmt.Sprintf("$%.2f", r.CostUSD)
			if r.CostUSD == 0 {
				cost = "-"
			}
			rows = append(rows, []string{phaseName, string(r.Status), cost, r.Duration})
		} else {
			rows = append(rows, []string{phaseName, "pending", "-", "-"})
		}
	}

	printer.Table(headers, rows)

	// Total cost
	var totalCost float64
	for _, r := range ws.Phases {
		totalCost += r.CostUSD
	}
	if totalCost > 0 {
		fmt.Printf("\nTotal cost: $%.2f\n", totalCost)
	}

	return nil
}

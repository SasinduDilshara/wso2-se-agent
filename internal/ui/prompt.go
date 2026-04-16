package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func (p *Printer) PauseForReview(phaseName string) bool {
	fmt.Printf("  %s\u25b8%s  pause \u2014 review %s results. Continue? [Y/n] ", Yellow, Reset, phaseName)
	return askYesNo(true)
}

func (p *Printer) ConfirmProceed(message string) bool {
	fmt.Printf("  %s [Y/n] ", message)
	return askYesNo(true)
}

func askYesNo(defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(strings.ToLower(text))

	if text == "" {
		return defaultYes
	}
	return text == "y" || text == "yes"
}

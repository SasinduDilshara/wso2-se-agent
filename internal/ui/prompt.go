package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func (p *Printer) PauseForReview(phaseName string) bool {
	fmt.Printf("  %s\u25b8%s  pause \u2014 review %s results. Continue? [y/n] ", Yellow, Reset, phaseName)
	return askYesNo()
}

func (p *Printer) ConfirmProceed(message string) bool {
	fmt.Printf("  %s [y/n] ", message)
	return askYesNo()
}

func askYesNo() bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(strings.ToLower(text))

		if text == "" {
			fmt.Print("  Please enter y or n: ")
			continue
		}
		return text == "y" || text == "yes"
	}
}

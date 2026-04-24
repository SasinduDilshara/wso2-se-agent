package main

import (
	"errors"
	"fmt"
	"os"

	wso2seagent "github.com/Tharsanan1/wso2-se-agent"
	"github.com/Tharsanan1/wso2-se-agent/internal/cmd"
	"github.com/Tharsanan1/wso2-se-agent/internal/config"
	"github.com/Tharsanan1/wso2-se-agent/internal/phase"
)

var version = "dev"

// Exit codes:
//
//	0 = success
//	1 = genuine failure (bad flags, config error, phase failure, etc.)
//	2 = risk gate halted the pipeline and is waiting for human review —
//	    the pipeline did exactly what it was supposed to do, but scripts
//	    and CI need a non-zero signal so they don't treat "REVIEW
//	    REQUIRED" as a pass.
const (
	exitGenuineFailure = 1
	exitReviewRequired = 2
)

func main() {
	config.ProductsFS = wso2seagent.ProductsFS

	err := cmd.Execute(version)
	if err == nil {
		return
	}
	if errors.Is(err, phase.ErrRiskGateHalt) {
		// The engine and printer have already produced the full user-facing
		// notice (header, reason excerpt, artifact path, resume command).
		// Printing `Error: …` on top of that would misframe a designed halt
		// as a failure — exit quietly with code 2 instead.
		os.Exit(exitReviewRequired)
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(exitGenuineFailure)
}

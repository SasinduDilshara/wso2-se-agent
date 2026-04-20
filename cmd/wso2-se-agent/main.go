package main

import (
	"fmt"
	"os"

	wso2seagent "github.com/SasinduDilshara/wso2-se-agent"
	"github.com/SasinduDilshara/wso2-se-agent/internal/cmd"
	"github.com/SasinduDilshara/wso2-se-agent/internal/config"
)

var version = "dev"

func main() {
	config.ProductsFS = wso2seagent.ProductsFS

	if err := cmd.Execute(version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

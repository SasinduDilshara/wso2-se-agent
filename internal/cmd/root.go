package cmd

import (
	"github.com/spf13/cobra"
)

var (
	appVersion string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "wso2-se-agent",
	Short: "Automate resolving GitHub issues using AI agents",
	Long:  "A CLI that developers can use to automate resolving GitHub issues on a product using AI agents.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute(version string) error {
	appVersion = version
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(setupReposCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("wso2-se-agent %s\n", appVersion)
	},
}

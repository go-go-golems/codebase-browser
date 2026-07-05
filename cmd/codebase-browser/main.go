package main

import (
	"os"

	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/spf13/cobra"

	"github.com/wesen/codebase-browser/cmd/codebase-browser/cmds/doc"
	"github.com/wesen/codebase-browser/cmd/codebase-browser/cmds/history"
	"github.com/wesen/codebase-browser/cmd/codebase-browser/cmds/index"
	"github.com/wesen/codebase-browser/cmd/codebase-browser/cmds/query"
	"github.com/wesen/codebase-browser/cmd/codebase-browser/cmds/review"
	"github.com/wesen/codebase-browser/cmd/codebase-browser/cmds/serve"
	"github.com/wesen/codebase-browser/cmd/codebase-browser/cmds/symbol"
	"github.com/wesen/codebase-browser/internal/concepts"
	helpdocs "github.com/wesen/codebase-browser/pkg/doc"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "codebase-browser",
	Short:   "Index, query, review, and export Go codebase data",
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromCobra(cmd)
	},
}

func main() {
	cobra.CheckErr(logging.AddLoggingSectionToRootCommand(rootCmd, "codebase-browser"))

	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	cobra.CheckErr(index.Register(rootCmd))
	cobra.CheckErr(symbol.Register(rootCmd))
	conceptRepositoryFlags := concepts.ExtractRepositoryFlagValuesFromArgs(os.Args[1:])
	cobra.CheckErr(query.Register(rootCmd, conceptRepositoryFlags))
	cobra.CheckErr(doc.Register(rootCmd))
	cobra.CheckErr(history.Register(rootCmd))
	cobra.CheckErr(review.Register(rootCmd))
	cobra.CheckErr(serve.Register(rootCmd))
	cobra.CheckErr(helpdocs.AddDocToHelpSystem(helpSystem))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

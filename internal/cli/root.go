package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"miocli/internal/app"
	"miocli/internal/config"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

type dependencies struct {
	terminal func(*cobra.Command) bool
	ask      func(*cobra.Command, string, string, bool) (string, error)
	choose   func(*cobra.Command, string, string, []choice) (string, error)
	probe    func(context.Context, config.Config) error
	ui       func(context.Context, config.Config, bool, app.Runtime) error
}

func Execute(ctx context.Context) error {
	return newCommand(defaultDependencies()).ExecuteContext(ctx)
}

type commandOptions struct {
	configPath string
	server     string
	noColor    bool
}

func newCommand(deps dependencies) *cobra.Command {
	options := &commandOptions{}
	load := options.loader()
	run := newConsoleRunner(deps, load, options)
	root := &cobra.Command{
		Use: "miocli", Short: "Terminal client for an existing Mihomo instance",
		Long:    "A keyboard-first client for an existing Mihomo instance.\nBrowse groups, select nodes, manage local subscription profiles, and update providers.\nConnect to an already running controller; miocli never installs or manages core processes.",
		Example: "  miocli setup                         # Configure a connection\n  miocli doctor                        # Run read-only checks\n  miocli profile list                  # List downloaded profiles\n  miocli dev                           # Use repository-local settings\n  miocli --server https://host:9090     # Connect to an existing controller",
		Args:    cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error { return run(cmd, false) },
	}
	root.SetIn(os.Stdin)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.PersistentFlags().StringVar(&options.configPath, "config", "", "path to YAML configuration")
	root.PersistentFlags().StringVar(&options.server, "server", "", "Mihomo control API URL, including unix:///absolute/socket (connect only for this invocation)")
	root.PersistentFlags().BoolVar(&options.noColor, "no-color", false, "disable terminal colors")
	root.AddCommand(newConnectionCommands(deps, load, run)...)
	root.AddCommand(newProfileCommand(load), newSystemProxyCommand(load), &cobra.Command{
		Use: "version", Short: "Print version information", Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "miocli %s (commit %s, built %s)\n", version, commit, buildDate)
		},
	})
	return root
}

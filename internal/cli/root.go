package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"miocli/internal/app"
	"miocli/internal/config"
	"miocli/internal/mihomo"
	"miocli/internal/theme"
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

func defaultDependencies() dependencies {
	return dependencies{
		terminal: isTerminal,
		ask:      askTerminal,
		choose:   chooseTerminal,
		probe:    probeConnection,
		ui: func(ctx context.Context, cfg config.Config, noColor bool, runtime app.Runtime) error {
			client, err := mihomo.NewHTTPClient(cfg.Server, cfg.Secret)
			if err != nil {
				return err
			}
			model := app.New(ctx, client, cfg.Server, noColor, app.WithRuntime(runtime))
			_, err = tea.NewProgram(model, tea.WithContext(ctx), tea.WithAltScreen()).Run()
			if err != nil {
				return fmt.Errorf("run terminal interface: %w", err)
			}
			return nil
		},
	}
}

func isTerminal(cmd *cobra.Command) bool {
	in, inOK := cmd.InOrStdin().(*os.File)
	out, outOK := cmd.OutOrStdout().(*os.File)
	return inOK && outOK && term.IsTerminal(in.Fd()) && term.IsTerminal(out.Fd())
}

func probeConnection(ctx context.Context, cfg config.Config) error {
	client, err := mihomo.NewHTTPClient(cfg.Server, cfg.Secret)
	if err != nil {
		return err
	}
	return mihomo.CheckConnection(ctx, client)
}

func newCommand(deps dependencies) *cobra.Command {
	var configPath, server string
	var noColor bool
	load := func(cmd *cobra.Command, dev bool) (config.Resolved, error) {
		path := configPath
		if dev && path == "" {
			dir, err := os.Getwd()
			if err != nil {
				return config.Resolved{}, err
			}
			path, err = config.DevelopmentPath(dir)
			if err != nil {
				return config.Resolved{}, err
			}
		}
		resolved, err := config.Resolve(config.Overrides{
			ConfigPath: path, Server: server, ServerSet: cmd.Flags().Changed("server"),
		})
		if dev && err != nil {
			return config.Resolved{}, fmt.Errorf("development configuration: %w; copy config.local.example.yaml to config.local.yaml, fill controller/secret, and on Unix use chmod 600", err)
		}
		return resolved, err
	}
	run := func(cmd *cobra.Command, dev bool) error {
		cfg, err := load(cmd, dev)
		if err != nil {
			return err
		}
		if err := deps.probe(cmd.Context(), cfg.Config); err != nil {
			if !dev && !cfg.Exists && !cfg.ExplicitServer && deps.terminal(cmd) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Controller unavailable: %v\n", err)
				return runSetup(cmd, cfg, deps)
			}
			return fmt.Errorf("connect to %s: %w\n%s", cfg.Server, err, config.ConnectionHint(err))
		}
		if !deps.terminal(cmd) {
			return errors.New("the control panel requires an interactive terminal; use miocli doctor for read-only diagnostics")
		}
		path, err := filepath.Abs(cfg.Path)
		if err != nil {
			return fmt.Errorf("resolve UI configuration path: %w", err)
		}
		return deps.ui(cmd.Context(), cfg.Config, theme.NoColor(noColor), app.Runtime{ConfigPath: path})
	}
	root := &cobra.Command{
		Use: "miocli", Short: "Terminal operations console for an existing Mihomo instance",
		Long:    "A keyboard-first terminal operations console for one existing Mihomo instance.\nBrowse groups, select nodes, test latency, and update proxy providers.\nConnect to an already running controller; miocli never installs, starts or stops cores.",
		Example: "  miocli setup                         # Configure a connection\n  miocli doctor                        # Run read-only checks\n  miocli dev                           # Use repository-local settings\n  miocli --server https://host:9090     # Connect to an existing controller",
		Args:    cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error { return run(cmd, false) },
	}
	root.SetIn(os.Stdin)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to YAML configuration")
	root.PersistentFlags().StringVar(&server, "server", "", "Mihomo control API URL (connect only for this invocation)")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable terminal colors")
	root.AddCommand(&cobra.Command{
		Use: "dev", Short: "Connect using repository-local config.local.yaml", Args: cobra.NoArgs,
		Example: "  miocli dev\n  miocli dev --config ./config.local.yaml",
		RunE:    func(cmd *cobra.Command, _ []string) error { return run(cmd, true) },
	}, &cobra.Command{
		Use: "setup", Short: "Configure a connection to an existing controller", Args: cobra.NoArgs,
		Example: "  miocli setup\n  miocli setup --config /absolute/path/config.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !deps.terminal(cmd) {
				return errors.New("setup requires an interactive terminal; run miocli setup in a terminal, or provision a connect configuration and MIOCLI_SERVER/MIOCLI_SECRET")
			}
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			return runSetup(cmd, cfg, deps)
		},
	}, &cobra.Command{
		Use: "doctor", Short: "Check configuration and controller without changing anything", Args: cobra.NoArgs,
		Example: "  miocli doctor\n  miocli doctor --config ./config.local.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			return runDoctor(cmd, cfg, deps)
		},
	}, &cobra.Command{
		Use: "version", Short: "Print version information", Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "miocli %s (commit %s, built %s)\n", version, commit, buildDate)
		},
	})
	return root
}

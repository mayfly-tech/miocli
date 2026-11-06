package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"miocli/internal/app"
	"miocli/internal/config"
	"miocli/internal/theme"
)

type configLoader func(*cobra.Command, bool) (config.Resolved, error)
type consoleRunner func(*cobra.Command, bool) error

func (options *commandOptions) loader() configLoader {
	return func(cmd *cobra.Command, dev bool) (config.Resolved, error) {
		path := options.configPath
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
			ConfigPath: path, Server: options.server, ServerSet: cmd.Flags().Changed("server"),
		})
		if dev && err != nil {
			return config.Resolved{}, fmt.Errorf("development configuration: %w; copy config.local.example.yaml to config.local.yaml, fill controller/secret, and on Unix use chmod 600", err)
		}
		return resolved, err
	}
}

func newConsoleRunner(deps dependencies, load configLoader, options *commandOptions) consoleRunner {
	return func(cmd *cobra.Command, dev bool) error {
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
		return deps.ui(cmd.Context(), cfg.Config, theme.NoColor(options.noColor), app.Runtime{ConfigPath: path})
	}
}

func newConnectionCommands(deps dependencies, load configLoader, run consoleRunner) []*cobra.Command {
	return []*cobra.Command{{
		Use: "dev", Short: "Connect using repository-local config.local.yaml", Args: cobra.NoArgs,
		Example: "  miocli dev\n  miocli dev --config ./config.local.yaml",
		RunE:    func(cmd *cobra.Command, _ []string) error { return run(cmd, true) },
	}, {
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
	}, {
		Use: "doctor", Short: "Check configuration and controller without changing anything", Args: cobra.NoArgs,
		Example: "  miocli doctor\n  miocli doctor --config ./config.local.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			return runDoctor(cmd, cfg, deps)
		},
	},
	}
}

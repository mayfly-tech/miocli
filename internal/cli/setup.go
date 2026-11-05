package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"miocli/internal/config"
)

func askChoice(cmd *cobra.Command, deps dependencies, label, initial string, choices ...choice) (string, error) {
	for {
		value, err := deps.choose(cmd, label, initial, choices)
		if err != nil {
			return "", err
		}
		for _, choice := range choices {
			if strings.EqualFold(value, choice.value) {
				return choice.value, nil
			}
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "Invalid selection; choose one of the displayed options.")
	}
}

func confirm(cmd *cobra.Command, deps dependencies, label string) error {
	value, err := askChoice(cmd, deps, label, "n", choice{"n", "Cancel / return to setup"}, choice{"y", "Confirm"})
	if err != nil {
		return err
	}
	if value != "y" {
		return errBack
	}
	return nil
}

func askServer(cmd *cobra.Command, deps dependencies, initial string) (string, error) {
	for {
		value, err := deps.ask(cmd, "Control API URL (not the HTTP/SOCKS proxy port)", initial, false)
		if err != nil {
			return "", err
		}
		server, err := config.ValidateServer(value)
		if err == nil {
			return server, nil
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Invalid controller: %v\n", err)
	}
}

func runSetup(cmd *cobra.Command, resolved config.Resolved, deps dependencies) error {
	if !deps.terminal(cmd) {
		return errors.New("setup requires an interactive terminal; run miocli setup in a terminal")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Setup connects to an already running Mihomo controller and saves private connection settings to %s.\nExisting Clash Verge and other client settings will not be modified. miocli never installs, starts or stops cores.\n", resolved.Path)
	if resolved.ExplicitServer || resolved.SecretSource == "MIOCLI_SECRET" {
		fmt.Fprintln(cmd.ErrOrStderr(), "Environment/--server overrides are active. Setup saves only values entered here; environment variables will still override saved settings on future runs.")
	}
	baseline := config.Config{Server: config.DefaultServer}
	if resolved.Exists {
		var err error
		baseline, err = config.Load(config.Overrides{
			ConfigPath: resolved.Path,
			LookupEnv: func(key string) (string, bool) {
				if key == "MIOCLI_SERVER" || key == "MIOCLI_SECRET" {
					return "", false
				}
				return os.LookupEnv(key)
			},
		})
		if err != nil {
			return err
		}
	}
	for {
		err := setupConnection(cmd, resolved.Path, baseline, deps)
		if errors.Is(err, errBack) {
			continue
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Setup complete. Private settings saved to %s.\nRun miocli to open the panel, or miocli doctor to check the connection.\n", resolved.Path)
		return nil
	}
}

func setupConnection(cmd *cobra.Command, path string, baseline config.Config, deps dependencies) error {
	for {
		cfg := baseline
		selected, err := askChoice(cmd, deps, "Connection details", "1",
			choice{"1", "Enter controller URL and secret"},
			choice{"2", "Read a specified Mihomo YAML (read-only)"},
			choice{"c", "Cancel setup"})
		if err != nil {
			return err
		}
		if selected == "c" {
			return context.Canceled
		}
		if selected == "2" {
			source, err := deps.ask(cmd, "Mihomo YAML path (read controller/secret only; never modify it)", "", false)
			if err != nil {
				return err
			}
			data, err := config.ReadConfigFile(source)
			if err == nil {
				cfg, err = config.ConnectionFromYAML(data)
			}
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Cannot import connection: %v\nEnter the actual loopback controller URL manually; socket/pipe is not supported.\n", err)
				continue
			}
		}
		cfg.Server, err = askServer(cmd, deps, cfg.Server)
		if err != nil {
			return err
		}
		cfg.Secret, err = deps.ask(cmd, "Controller secret (hidden; empty means no authentication)", cfg.Secret, true)
		if err != nil {
			return err
		}
		if err := runWork(cmd, "Checking read-only controller APIs", func(ctx context.Context) error {
			return deps.probe(ctx, cfg)
		}); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Connection failed: %v\n%s\n", err, config.ConnectionHint(err))
			baseline = cfg
			continue
		}
		if err := confirm(cmd, deps, fmt.Sprintf("Save connection settings to %s? Secret is stored in protected YAML, not an OS keychain.", path)); err != nil {
			return err
		}
		return config.Save(path, cfg)
	}
}

func runDoctor(cmd *cobra.Command, cfg config.Resolved, deps dependencies) error {
	secret := "not set"
	if cfg.Secret != "" {
		secret = "set (hidden)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Configuration\n  Config: %s (exists: %t)\n  Server: %s (source: %s)\n  Secret: %s (source: %s)\n\nController\n",
		cfg.Path, cfg.Exists, cfg.Server, cfg.ServerSource, secret, cfg.SecretSource)
	if err := deps.probe(cmd.Context(), cfg.Config); err != nil {
		return fmt.Errorf("controller check failed: %w\n%s", err, config.ConnectionHint(err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "  OK: /version, /proxies, /configs\n\nNo settings or processes were changed.")
	return nil
}

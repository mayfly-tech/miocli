package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"miocli/internal/config"
	"miocli/internal/core"
	"miocli/internal/profile"
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

func askServer(cmd *cobra.Command, deps dependencies, initial string, managed bool) (string, error) {
	for {
		value, err := deps.ask(cmd, "Control API URL (not the HTTP/SOCKS proxy port)", initial, false)
		if err != nil {
			return "", err
		}
		server, err := config.ValidateServer(value)
		if err == nil && managed {
			err = core.ValidateManagedServer(server)
		}
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
	fmt.Fprintf(cmd.OutOrStdout(), "Setup saves private connection settings to %s.\nExisting Clash Verge and other client settings will not be modified.\n", resolved.Path)
	if resolved.ExplicitServer || resolved.SecretSource == "MIOCLI_SECRET" {
		fmt.Fprintln(cmd.ErrOrStderr(), "Environment/--server overrides are active. Setup saves only values entered here; environment variables will still override saved settings on future runs.")
	}
	baseline := config.Config{Server: config.DefaultServer, Core: config.Core{Mode: config.Connect}}
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
		choice, err := askChoice(cmd, deps, "How would you like to use Mihomo?", "1",
			choice{"1", "Connect to an existing core (recommended)"},
			choice{"2", "Manage a separate local core"},
			choice{"c", "Cancel setup"})
		if errors.Is(err, errBack) {
			continue
		}
		if err != nil {
			return err
		}
		if choice == "c" {
			return context.Canceled
		}
		if choice == "1" {
			err = setupConnection(cmd, resolved.Path, baseline, deps)
		} else {
			err = setupManaged(cmd, resolved.Path, baseline, deps)
		}
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
		cfg := config.Config{Server: baseline.Server, Secret: baseline.Secret, Core: config.Core{Mode: config.Connect}}
		choice, err := askChoice(cmd, deps, "Connection details", "1",
			choice{"1", "Enter controller URL and secret"},
			choice{"2", "Read a specified Mihomo YAML (read-only)"})
		if err != nil {
			return err
		}
		if choice == "2" {
			source, err := deps.ask(cmd, "Mihomo YAML path (read controller/secret only; never modify it)", "", false)
			if err != nil {
				return err
			}
			data, err := core.ReadConfigFile(source)
			if err == nil {
				cfg, err = core.ConnectionFromYAML(data)
			}
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Cannot import connection: %v\nEnter the actual loopback controller URL manually; socket/pipe is not supported.\n", err)
				continue
			}
		}
		cfg.Server, err = askServer(cmd, deps, cfg.Server, false)
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
			fmt.Fprintf(cmd.ErrOrStderr(), "Connection failed: %v\n%s\n", err, core.ConnectionHint(err))
			baseline = cfg
			continue
		}
		if err := confirm(cmd, deps, fmt.Sprintf("Save connect settings to %s? Secret is stored in protected YAML, not an OS keychain.", path)); err != nil {
			return err
		}
		return config.Save(path, cfg)
	}
}

func setupManaged(cmd *cobra.Command, path string, baseline config.Config, deps dependencies) error {
	paths, err := core.PathsFor(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Managed mode creates a copy at %s, starts a child core for the panel, and stops only that child on exit.\nIt never changes system proxy, services or TUN permissions. Import copies with TUN/custom listeners disabled; relative resources must be available to the copy.\n", paths.ConfigDir)
	if err := confirm(cmd, deps, "Allow configuring and testing a managed core (including starting a temporary child)?"); err != nil {
		return err
	}
	server, err := askServer(cmd, deps, baseline.Server, true)
	if err != nil {
		return err
	}
	selected, err := askChoice(cmd, deps, "Import configuration", "1",
		choice{"1", "Local YAML copy"}, choice{"2", "HTTPS subscription"})
	if err != nil {
		return err
	}
	source, err := deps.ask(cmd, "Source path or HTTPS URL (subscription input is hidden)", "", selected == "2")
	if err != nil {
		return err
	}
	var data []byte
	if selected == "1" {
		data, err = core.ReadConfigFile(source)
	} else {
		err = runWork(cmd, "Downloading subscription configuration", func(ctx context.Context) error {
			var downloadErr error
			data, downloadErr = core.DownloadConfig(ctx, strings.TrimSpace(source))
			return downloadErr
		})
	}
	if err != nil {
		return err
	}
	raw := append([]byte(nil), data...)
	kind := "https"
	if selected == "1" {
		kind = "file"
		source, err = core.ExpandHome(source)
		if err == nil {
			source, err = filepath.Abs(source)
		}
		if err != nil {
			return err
		}
	}
	secret, data, err := core.PrepareManagedConfig(data, server, "")
	if err != nil {
		return err
	}
	explicit, err := deps.ask(cmd, "Mihomo executable path (empty: search PATH and miocli's private directory, not GUI bundles)", baseline.Core.Binary, false)
	if err != nil {
		return err
	}
	if explicit != "" {
		explicit, err = core.ExpandHome(explicit)
		if err != nil {
			return err
		}
	}
	binary, err := core.FindMihomo(paths.BinaryDir, explicit)
	if err != nil {
		if explicit != "" || !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "No reusable core found. Download target: %s.\n", paths.BinaryDir)
		version, err := core.PinnedMihomoVersion()
		if err != nil {
			return err
		}
		if err := confirm(cmd, deps, fmt.Sprintf("Download Mihomo %s from GitHub (domestic S3 fallback on network/HTTP failure) and verify pinned SHA-256?", version)); err != nil {
			return err
		}
		var events []core.DownloadProgress
		err = runWork(cmd, "Downloading and verifying Mihomo", func(ctx context.Context) error {
			var installErr error
			binary, installErr = core.InstallMihomoWithProgress(ctx, paths.BinaryDir, func(event core.DownloadProgress) {
				events = append(events, event)
			})
			return installErr
		})
		for _, event := range events {
			if event.Fallback {
				fmt.Fprintf(cmd.ErrOrStderr(), "GitHub unavailable: %s. Retrying the same Mihomo version from domestic S3.\n", event.Reason)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Verified Mihomo %s from %s.\n", version, event.Source)
			}
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Downloaded core retained at %s, even if later setup is cancelled or fails.\n", binary)
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	version, err := core.BinaryVersion(cmd.Context(), binary)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Core: %s\nVersion: %s\nController: %s\nInherited TLS/socket/pipe/DoH control listeners are removed only from the managed copy.\n", binary, version, server)
	if err := confirm(cmd, deps, "Validate, briefly start and save this managed configuration? Existing managed files will be replaced only on success."); err != nil {
		return err
	}
	cfg := config.Config{Server: server, Secret: secret, Core: config.Core{Mode: config.Managed, Binary: binary}}
	return runWork(cmd, "Validating managed configuration and startup", func(ctx context.Context) error {
		return core.SetupManaged(ctx, cfg, paths, data, func() error {
			return profile.New(paths.AppConfig, cfg, false, nil).Register("Default", kind, strings.TrimSpace(source), raw)
		})
	})
}

func runDoctor(cmd *cobra.Command, cfg config.Resolved, deps dependencies) error {
	secret := "not set"
	if cfg.Secret != "" {
		secret = "set (hidden)"
	}
	mode := cfg.Core.Mode
	if cfg.ExplicitServer {
		mode = config.Connect + " (address override; no managed startup)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Configuration\n  Config: %s (exists: %t)\n  Server: %s (source: %s)\n  Secret: %s (source: %s)\n\nCore\n  Mode: %s\n",
		cfg.Path, cfg.Exists, cfg.Server, cfg.ServerSource, secret, cfg.SecretSource, mode)
	var coreErr error
	if cfg.Core.Mode == config.Managed {
		paths, err := core.PathsFor(cfg.Path)
		if err == nil {
			var binary string
			binary, err = core.FindMihomo(paths.BinaryDir, cfg.Core.Binary)
			if err == nil {
				var version string
				version, err = core.BinaryVersion(cmd.Context(), binary)
				if err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Binary: %s\n  Version: %s\n", binary, version)
				}
			}
		}
		coreErr = err
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Core check failed: %v\n", err)
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nController")
	if err := deps.probe(cmd.Context(), cfg.Config); err != nil {
		return errors.Join(coreErr, fmt.Errorf("controller check failed: %w\n%s", err, core.ConnectionHint(err)))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "  OK: /version, /proxies, /configs\n\nNo settings or processes were changed.")
	return coreErr
}

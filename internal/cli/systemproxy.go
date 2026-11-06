package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"miocli/internal/config"
	"miocli/internal/mihomo"
	"miocli/internal/systemproxy"
)

func newSystemProxyCommand(load configLoader) *cobra.Command {
	systemProxy := &cobra.Command{
		Use:   "system-proxy",
		Short: "Control the current user's system proxy using local Mihomo ports",
		Args:  cobra.NoArgs,
	}
	systemProxy.AddCommand(&cobra.Command{
		Use: "status", Short: "Show the operating system proxy status", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, _ := cmd.Flags().GetString("service")
			status, err := systemproxy.Status(service)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), status)
			return nil
		},
	}, &cobra.Command{
		Use: "enable", Short: "Set the system proxy to local Mihomo ports", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			if !config.IsLocalController(cfg.Server) {
				return errors.New("system proxy requires a local Mihomo controller and local proxy ports")
			}
			client, err := mihomo.NewHTTPClient(cfg.Server, cfg.Secret)
			if err != nil {
				return err
			}
			runtimeConfig, err := client.RuntimeConfig(cmd.Context())
			if err != nil {
				return fmt.Errorf("read Mihomo proxy ports: %w", err)
			}
			switch runtimeConfig.BindAddress {
			case "", "*", "0.0.0.0", "::", "127.0.0.1", "::1":
			default:
				return fmt.Errorf("Mihomo bind-address %q does not expose a loopback proxy listener; configure a loopback-compatible bind-address first", runtimeConfig.BindAddress)
			}
			httpPort := runtimeConfig.MixedPort
			if httpPort == 0 {
				httpPort = runtimeConfig.Port
			}
			socksPort := runtimeConfig.MixedPort
			if socksPort == 0 {
				socksPort = runtimeConfig.SocksPort
			}
			service, _ := cmd.Flags().GetString("service")
			endpoint := systemproxy.Endpoint{Host: "127.0.0.1", HTTPPort: httpPort, SOCKSPort: socksPort}
			if err := systemproxy.Enable(endpoint, service, cfg.Path); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "System proxy enabled. Use `miocli system-proxy disable` to restore the previous settings.")
			return nil
		},
	}, &cobra.Command{
		Use: "disable", Short: "Restore the system proxy settings saved by miocli", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			service, _ := cmd.Flags().GetString("service")
			if err := systemproxy.Disable(service, cfg.Path); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Previous system proxy settings restored.")
			return nil
		},
	})
	for _, command := range systemProxy.Commands() {
		command.Flags().String("service", "", "macOS network service name (see networksetup -listnetworkserviceorder)")
	}
	return systemProxy
}

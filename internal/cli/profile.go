package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"miocli/internal/mihomo"
	"miocli/internal/profiles"
)

func newProfileCommand(load configLoader) *cobra.Command {
	profile := &cobra.Command{
		Use:   "profile",
		Short: "Manage HTTPS subscription profiles for a local Mihomo instance",
		Long:  "Download and store Mihomo YAML profiles, then ask a local controller to reload one. Profile files are stored privately beside the miocli settings.",
		Args:  cobra.NoArgs,
	}
	profile.AddCommand(&cobra.Command{
		Use: "list", Short: "List stored profiles without revealing subscription URLs", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			entries, err := profiles.New(cfg.Path).List()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No profiles configured.")
				return nil
			}
			for _, entry := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\tupdated %s\n", entry.Name, entry.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}, &cobra.Command{
		Use: "add NAME HTTPS_URL", Short: "Download and save a subscription profile", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			if err := profiles.New(cfg.Path).Add(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Profile %s downloaded. Activate it with: miocli profile use %s\n", args[0], args[0])
			return nil
		},
	}, &cobra.Command{
		Use: "update NAME", Short: "Download the latest profile content", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			if err := profiles.New(cfg.Path).Update(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Profile %s updated; activate it with: miocli profile use %s\n", args[0], args[0])
			return nil
		},
	}, &cobra.Command{
		Use: "use NAME", Short: "Reload Mihomo using a stored profile", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			client, err := mihomo.NewHTTPClient(cfg.Server, cfg.Secret)
			if err != nil {
				return err
			}
			if err := profiles.New(cfg.Path).Activate(cmd.Context(), args[0], cfg.Path, cfg.Config, client); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mihomo reload requested for profile %s. Verify the active state with miocli doctor.\n", args[0])
			return nil
		},
	}, &cobra.Command{
		Use: "remove NAME", Short: "Remove a stored profile and its local file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load(cmd, false)
			if err != nil {
				return err
			}
			if err := profiles.New(cfg.Path).Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Profile %s removed.\n", args[0])
			return nil
		},
	})
	return profile
}

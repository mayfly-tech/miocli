package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"miocli/internal/app"
	"miocli/internal/config"
	"miocli/internal/mihomo"
)

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

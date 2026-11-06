package profiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"miocli/internal/config"
)

// Reloader accepts a local profile path on the controller host.
type Reloader interface {
	ReloadConfig(context.Context, string) error
}

// Activate prepares a private effective profile, reloads it, and optionally saves
// the restart configuration. A failed reload retains the effective file because
// the controller may still be processing it.
func (m *Manager) Activate(ctx context.Context, name, settingsPath string, cfg config.Config, client Reloader) error {
	if !config.IsLocalController(cfg.Server) {
		return errors.New("profile activation requires a local Mihomo controller because the profile path must exist on the core's host")
	}
	path, err := m.ConfigPath(name)
	if err != nil {
		return err
	}
	data, err := config.ReadConfigFile(path)
	if err != nil {
		return err
	}
	data, err = config.ProfileWithController(data, cfg)
	if err != nil {
		return fmt.Errorf("prepare Mihomo profile: %w", err)
	}
	if cfg.ActiveProfilePath != "" {
		absoluteSettingsPath, err := filepath.Abs(settingsPath)
		if err != nil {
			return fmt.Errorf("resolve miocli settings path: %w", err)
		}
		if filepath.Clean(cfg.ActiveProfilePath) == absoluteSettingsPath {
			return errors.New("active-profile-path must not overwrite the miocli settings")
		}
		if filepath.Clean(cfg.ActiveProfilePath) == filepath.Clean(path) {
			return errors.New("active-profile-path must not overwrite the original subscription profile")
		}
	}
	effective, err := os.CreateTemp(filepath.Dir(path), ".active-*.yaml")
	if err != nil {
		return fmt.Errorf("create effective profile: %w", err)
	}
	effectivePath := effective.Name()
	keepEffective := false
	defer func() {
		if !keepEffective {
			os.Remove(effectivePath)
		}
	}()
	if err := effective.Close(); err != nil {
		return fmt.Errorf("close effective profile: %w", err)
	}
	if err := config.WritePrivate(effectivePath, data); err != nil {
		return fmt.Errorf("write effective profile: %w", err)
	}
	if err := client.ReloadConfig(ctx, effectivePath); err != nil {
		keepEffective = true
		return fmt.Errorf("reload Mihomo profile: %w; runtime state may have changed; restart configuration was not saved; effective profile retained at %s; inspect core logs and controller state before retrying", err, effectivePath)
	}
	if cfg.ActiveProfilePath != "" {
		if err := config.WritePrivate(cfg.ActiveProfilePath, data); err != nil {
			return fmt.Errorf("profile reloaded, but saving the restart configuration failed: %w", err)
		}
	}
	return nil
}

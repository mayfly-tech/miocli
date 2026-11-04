package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"miocli/internal/config"
	"miocli/internal/mihomo"
)

func connectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

func ValidateManagedServer(server string) error {
	if _, err := config.ValidateServer(server); err != nil {
		return err
	}
	parsed, err := url.Parse(server)
	if err != nil {
		return err
	}
	ip := net.ParseIP(parsed.Hostname())
	if parsed.Scheme != "http" || ip == nil || !ip.IsLoopback() || parsed.Port() == "" {
		return errors.New("managed controller requires an HTTP URL with a literal loopback IP and an explicit port")
	}
	return nil
}

func CheckManagedConfig(ctx context.Context, binary, dir string, data []byte, cfg config.Config) error {
	if err := ValidateManagedServer(cfg.Server); err != nil {
		return err
	}
	document, err := yamlMapping(data)
	if err != nil {
		return err
	}
	root := document.Content[0]
	parsed, err := url.Parse(cfg.Server)
	if err != nil {
		return err
	}
	if mappingScalar(root, "external-controller") != parsed.Host || mappingScalar(root, "secret") != cfg.Secret {
		return errors.New("managed controller/secret differs from connection settings; clear environment overrides or run miocli setup")
	}
	var fields map[string]any
	if err := document.Decode(&fields); err != nil {
		return errors.New("invalid managed configuration")
	}
	if tun, ok := fields["tun"].(map[string]any); ok {
		if enabled, ok := tun["enable"].(bool); !ok && tun["enable"] != nil || enabled {
			return errors.New("managed configuration enables TUN; disable it in the source copy before import (miocli never elevates privileges)")
		}
	}
	if listeners, ok := fields["listeners"]; ok && listeners != nil {
		if items, ok := listeners.([]any); !ok || len(items) != 0 {
			return errors.New("managed setup cannot preflight custom listeners; remove them from the import copy and use standard port settings")
		}
	}
	for _, key := range []string{"external-controller-tls", "external-controller-unix", "external-controller-pipe", "external-doh-server"} {
		if mappingScalar(root, key) != "" {
			return fmt.Errorf("managed configuration must not enable %s; run miocli setup", key)
		}
	}
	if err := checkListener("tcp", parsed.Host, "controller"); err != nil {
		return err
	}
	host := "127.0.0.1"
	if lan, ok := fields["allow-lan"].(bool); ok && lan {
		host = "0.0.0.0"
		if bind, ok := fields["bind-address"].(string); ok && bind != "" && bind != "*" {
			host = bind
		}
	}
	for _, key := range []string{"port", "mixed-port", "socks-port", "redir-port", "tproxy-port"} {
		value, ok := fields[key]
		if !ok {
			continue
		}
		port, err := strconv.Atoi(fmt.Sprint(value))
		if err != nil || port < 0 || port > 65535 {
			return fmt.Errorf("managed %s must be a port between 0 and 65535", key)
		}
		if port == 0 {
			continue
		}
		address := net.JoinHostPort(host, strconv.Itoa(port))
		if err := checkListener("tcp", address, key); err != nil {
			return err
		}
		if key != "port" && key != "redir-port" {
			if err := checkListener("udp", address, key); err != nil {
				return err
			}
		}
	}
	if dns, ok := fields["dns"].(map[string]any); ok {
		if enabled, _ := dns["enable"].(bool); enabled {
			if address, ok := dns["listen"].(string); ok && address != "" {
				for _, network := range []string{"tcp", "udp"} {
					if err := checkListener(network, address, "DNS"); err != nil {
						return err
					}
				}
			}
		}
	}
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	logPath := filepath.Join(dir, "config-check.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create config check log: %w", err)
	}
	if err := config.ProtectFile(logPath); err != nil {
		log.Close()
		return err
	}
	command := exec.CommandContext(checkCtx, binary, "-t", "-d", dir)
	command.Stdout, command.Stderr = log, log
	runErr := command.Run()
	closeErr := log.Close()
	if runErr != nil {
		return fmt.Errorf("Mihomo configuration check failed; check resources/relative paths and inspect %s: %w", logPath, errors.Join(runErr, closeErr))
	}
	if closeErr != nil {
		return fmt.Errorf("close config check log: %w", closeErr)
	}
	return nil
}

func checkListener(network, address, label string) error {
	if network == "udp" {
		listener, err := net.ListenPacket(network, address)
		if err != nil {
			return fmt.Errorf("%s %s listener %s is unavailable; change the imported copy, not the existing core: %w", label, network, address, err)
		}
		return listener.Close()
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return fmt.Errorf("%s %s listener %s is unavailable; choose another port: %w", label, network, address, err)
	}
	return listener.Close()
}

func SetupManaged(ctx context.Context, cfg config.Config, paths Paths, data []byte, saveProfile ...func() error) (resultErr error) {
	if cfg.Core.Mode != config.Managed {
		return errors.New("managed setup requires core.mode: managed")
	}
	if len(saveProfile) > 1 {
		return errors.New("managed setup accepts one profile commit")
	}
	previousConfig, previousErr := os.ReadFile(paths.AppConfig)
	if previousErr != nil && !errors.Is(previousErr, os.ErrNotExist) {
		return fmt.Errorf("read previous connection configuration: %w", previousErr)
	}
	if err := os.MkdirAll(filepath.Dir(paths.ConfigDir), 0700); err != nil {
		return fmt.Errorf("create managed parent directory: %w", err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(paths.ConfigDir), ".mihomo-setup-")
	if err != nil {
		return fmt.Errorf("create setup staging directory: %w", err)
	}
	defer func() {
		if resultErr != nil {
			for _, name := range []string{"config-check.log", "mihomo.log"} {
				data, err := os.ReadFile(filepath.Join(stage, name))
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("retain setup log: %w", err))
					continue
				}
				logPath := filepath.Join(filepath.Dir(paths.ConfigDir), "setup-"+name)
				if err := config.WritePrivate(logPath, data); err != nil {
					resultErr = errors.Join(resultErr, err)
				} else {
					resultErr = fmt.Errorf("%w; private diagnostic log retained at %s", resultErr, logPath)
				}
			}
		}
		resultErr = errors.Join(resultErr, os.RemoveAll(stage))
	}()
	if err := writeManagedConfig(filepath.Join(stage, "config.yaml"), data); err != nil {
		return err
	}
	if err := CheckManagedConfig(ctx, cfg.Core.Binary, stage, data, cfg); err != nil {
		return err
	}
	manager, err := startMihomo(ctx, cfg.Core.Binary, stage, cfg.Server, cfg.Secret)
	if err != nil {
		return err
	}
	if err := manager.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	backupDir := stage + "-previous"
	hadPrevious := false
	if info, err := os.Lstat(paths.ConfigDir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("managed config directory must be a real directory, not a symlink")
		}
		if err := os.Rename(paths.ConfigDir, backupDir); err != nil {
			return fmt.Errorf("back up previous managed configuration: %w", err)
		}
		hadPrevious = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect managed configuration directory: %w", err)
	}
	rollback := func(cause error) error {
		if hadPrevious {
			if err := os.Rename(backupDir, paths.ConfigDir); err != nil {
				return errors.Join(cause, fmt.Errorf("restore previous managed configuration from %s: %w", backupDir, err))
			}
		}
		return cause
	}
	if err := os.Rename(stage, paths.ConfigDir); err != nil {
		return rollback(fmt.Errorf("install managed configuration: %w", err))
	}
	if err := config.Save(paths.AppConfig, cfg); err != nil {
		if moveErr := os.Rename(paths.ConfigDir, stage); moveErr != nil {
			return errors.Join(err, fmt.Errorf("move failed setup aside; previous configuration retained at %s: %w", backupDir, moveErr))
		}
		return rollback(err)
	}
	if len(saveProfile) == 1 {
		if err := saveProfile[0](); err != nil {
			var restoreErr error
			if previousErr == nil {
				restoreErr = config.WritePrivate(paths.AppConfig, previousConfig)
			} else {
				restoreErr = os.Remove(paths.AppConfig)
			}
			if moveErr := os.Rename(paths.ConfigDir, stage); moveErr != nil {
				return errors.Join(err, restoreErr, fmt.Errorf("profile commit failed; previous runtime retained at %s: %w", backupDir, moveErr))
			}
			return errors.Join(rollback(err), restoreErr)
		}
	}
	if hadPrevious {
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("setup saved, but old managed backup could not be removed at %s: %w", backupDir, err)
		}
	}
	return nil
}

func ConnectionHint(err error) string {
	var apiErr *mihomo.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case mihomo.ErrorAuth:
			return "Authentication/permission denied: check the owning app's controller secret and MIOCLI_SECRET. No second core will be started."
		case mihomo.ErrorTimeout:
			return "Controller timed out: check the owning app and endpoint. No second core will be started."
		case mihomo.ErrorDecode, mihomo.ErrorNotFound:
			return "The endpoint is not a compatible Mihomo control API; check that it is not the proxy port."
		}
	}
	if connectionRefused(err) {
		return "Connection refused: start the owning app or check its controller port. Run miocli setup to configure it; no core was installed."
	}
	return "Check the control API (not the proxy port), secret, TLS certificate and owning app; use miocli setup or miocli doctor. No core was installed."
}

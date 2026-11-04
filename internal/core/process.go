package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"miocli/internal/config"
	"miocli/internal/mihomo"
)

const startupTimeout = 30 * time.Second

type Manager struct {
	command *exec.Cmd
	done    chan struct{}
	log     *os.File
	logPath string
	waitErr error
	close   sync.Once
}

func startMihomo(ctx context.Context, binary, configDir, server, secret string) (*Manager, error) {
	logPath := filepath.Join(configDir, "mihomo.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open Mihomo log: %w", err)
	}
	if err := config.ProtectFile(logPath); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("protect Mihomo log: %w", err)
	}
	command := exec.Command(binary, "-d", configDir)
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start Mihomo: %w", err)
	}
	manager := &Manager{
		command: command,
		done:    make(chan struct{}),
		log:     logFile,
		logPath: logPath,
	}
	go func() {
		manager.waitErr = command.Wait()
		close(manager.done)
	}()

	client, err := mihomo.NewHTTPClient(server, secret)
	if err != nil {
		if closeErr := manager.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	deadline := time.NewTimer(startupTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		probeCtx, cancel := context.WithTimeout(ctx, time.Second)
		probeErr := mihomo.CheckConnection(probeCtx, client)
		cancel()
		if probeErr == nil {
			return manager, nil
		}
		select {
		case <-ctx.Done():
			if closeErr := manager.Close(); closeErr != nil {
				return nil, errors.Join(ctx.Err(), closeErr)
			}
			return nil, ctx.Err()
		case <-manager.done:
			if closeErr := manager.Close(); closeErr != nil {
				return nil, fmt.Errorf("Mihomo exited before its controller became ready; inspect %s: %w", logPath, closeErr)
			}
			if manager.waitErr != nil {
				return nil, fmt.Errorf("Mihomo exited before its controller became ready; inspect %s: %w", logPath, manager.waitErr)
			}
			return nil, fmt.Errorf("Mihomo exited before its controller became ready; inspect %s", logPath)
		case <-deadline.C:
			if err := manager.Close(); err != nil {
				return nil, fmt.Errorf("Mihomo controller did not become ready; inspect %s: %w", logPath, err)
			}
			return nil, fmt.Errorf("Mihomo controller did not become ready; inspect %s", logPath)
		case <-ticker.C:
		}
	}
}

func (manager *Manager) Close() error {
	if manager == nil {
		return nil
	}
	var closeErr error
	manager.close.Do(func() {
		select {
		case <-manager.done:
			// The owned process has already exited.
		default:
			if err := manager.command.Process.Signal(os.Interrupt); err != nil {
				if killErr := manager.command.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
					closeErr = fmt.Errorf("stop Mihomo process: %w", killErr)
				}
			}
			select {
			case <-manager.done:
			case <-time.After(5 * time.Second):
				if err := manager.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					closeErr = fmt.Errorf("kill Mihomo process: %w", err)
				} else {
					select {
					case <-manager.done:
					case <-time.After(5 * time.Second):
						closeErr = errors.New("Mihomo process did not exit after being killed")
					}
				}
			}
		}
		if err := manager.log.Close(); closeErr == nil && err != nil {
			closeErr = fmt.Errorf("close Mihomo log: %w", err)
		}
	})
	return closeErr
}

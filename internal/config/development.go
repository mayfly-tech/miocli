package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func DevelopmentPath(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve development directory: %w", err)
	}
	for range 32 {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 2 && fields[0] == "module" && fields[1] == "miocli" {
				if _, err := os.Stat(filepath.Join(dir, "config.local.example.yaml")); err != nil {
					return "", fmt.Errorf("locate miocli development template: %w", err)
				}
				return filepath.Join(dir, "config.local.yaml"), nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read development module: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("dev must run inside the miocli source checkout, or use dev --config PATH")
}

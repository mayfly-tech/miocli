//go:build !windows

package config

import (
	"fmt"
	"os"
)

func ProtectFile(path string) error {
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("protect private file: %w", err)
	}
	return nil
}

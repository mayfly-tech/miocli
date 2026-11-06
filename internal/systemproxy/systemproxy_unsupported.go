//go:build !linux && !darwin && !windows

package systemproxy

import "errors"

func Enable(Endpoint, string, string) error {
	return errors.New("system proxy management is not supported on this operating system")
}

func Disable(string, string) error {
	return errors.New("system proxy management is not supported on this operating system")
}

func Status(string) (string, error) {
	return "", errors.New("system proxy management is not supported on this operating system")
}

package mihomo

import (
	"context"
	"errors"
)

func CheckConnection(ctx context.Context, client Client) error {
	version, err := client.Version(ctx)
	if err != nil {
		return err
	}
	if !version.Meta {
		return errors.New("controller did not identify itself as Mihomo (meta: true)")
	}
	if _, err := client.Proxies(ctx); err != nil {
		return err
	}
	mode, err := client.Mode(ctx)
	if err != nil {
		return err
	}
	if mode == "" {
		return errors.New("controller /configs response is missing mode")
	}
	return nil
}

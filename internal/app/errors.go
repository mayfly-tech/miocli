package app

import (
	"errors"

	"miocli/internal/mihomo"
)

func messageForError(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *mihomo.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case mihomo.ErrorAuth:
			return "Authentication failed; check the Mihomo secret"
		case mihomo.ErrorTimeout:
			return "Request timed out; check that Mihomo is reachable"
		case mihomo.ErrorNotFound:
			return "API unsupported or resource not found"
		case mihomo.ErrorDecode:
			return "Mihomo returned an unrecognized response"
		}
	}
	return err.Error()
}

func transientError(err error) bool {
	var apiErr *mihomo.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Kind == mihomo.ErrorNetwork ||
		apiErr.Kind == mihomo.ErrorTimeout ||
		(apiErr.Kind == mihomo.ErrorAPI && apiErr.Status >= 500)
}

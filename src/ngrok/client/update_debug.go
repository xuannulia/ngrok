package client

import (
	"ngrok/client/mvc"
)

// Self-managed forks do not use the legacy Equinox auto-update service.
func autoUpdate(state mvc.State, token string) {
}

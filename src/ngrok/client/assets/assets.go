package assets

import "embed"

//go:embed assets/client
var files embed.FS

func Asset(name string) ([]byte, error) {
	return files.ReadFile(name)
}

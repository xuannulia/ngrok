package assets

import "embed"

//go:embed assets/server
var files embed.FS

func Asset(name string) ([]byte, error) {
	return files.ReadFile(name)
}

package utils

import (
	"os"
	"path/filepath"
)

func ExpandTilde(path string) string {
	if path[0] != '~' {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return filepath.Join(home, path[1:])
}

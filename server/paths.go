package server

import (
	"path/filepath"
	"strings"
)

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func pathsOverlap(a, b string) bool {
	return pathWithin(a, b) || pathWithin(b, a)
}

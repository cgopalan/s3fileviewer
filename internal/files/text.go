package files

import (
	"path/filepath"
	"strings"
)

var allowedExtensions = map[string]struct{}{
	".csv": {},
	".txt": {},
}

func IsAllowedExtension(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	_, ok := allowedExtensions[ext]
	return ok
}

func HasBinaryContent(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func IsViewableTextFile(key string, sample []byte) bool {
	if !IsAllowedExtension(key) {
		return false
	}
	if len(sample) > 0 && HasBinaryContent(sample) {
		return false
	}
	return true
}

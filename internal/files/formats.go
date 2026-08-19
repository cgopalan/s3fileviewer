package files

import (
	"path/filepath"
	"strings"
)

var supportedExtensions = map[string]struct{}{
	".csv":     {},
	".txt":     {},
	".parquet": {},
	".zip":     {},
}

var queryableMemberExtensions = map[string]struct{}{
	".csv":     {},
	".txt":     {},
	".parquet": {},
}

func IsSupportedExtension(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	if ext == "" {
		return true
	}
	_, ok := supportedExtensions[ext]
	return ok
}

// IsAllowedExtension is kept for backward compatibility.
func IsAllowedExtension(key string) bool {
	return IsSupportedExtension(key)
}

func IsZipFile(key string) bool {
	return strings.ToLower(filepath.Ext(key)) == ".zip"
}

func IsBinaryQueryable(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	return ext == ".parquet" || ext == ".zip"
}

func IsTextFile(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	return ext == "" || ext == ".csv" || ext == ".txt"
}

func IsQueryableMember(member string) bool {
	ext := strings.ToLower(filepath.Ext(member))
	if ext == "" {
		return true
	}
	_, ok := queryableMemberExtensions[ext]
	return ok
}

func EffectiveExtension(key, member string) string {
	var ext string
	if member != "" {
		ext = strings.ToLower(filepath.Ext(member))
	} else {
		ext = strings.ToLower(filepath.Ext(key))
	}
	if ext == "" {
		return ".csv"
	}
	return ext
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
	if !IsTextFile(key) {
		return false
	}
	if len(sample) > 0 && HasBinaryContent(sample) {
		return false
	}
	return true
}

func IsOpenable(key string, sample []byte) bool {
	if !IsSupportedExtension(key) {
		return false
	}
	if IsBinaryQueryable(key) {
		return true
	}
	return IsViewableTextFile(key, sample)
}

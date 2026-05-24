package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port            string
	AWSRegion       string
	MaxQueryRows    int
	QueryTimeoutSec int
	RawPreviewBytes int64
	BucketAllowlist map[string]struct{}
}

func Load() (*Config, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		return nil, fmt.Errorf("AWS_REGION or AWS_DEFAULT_REGION is required")
	}

	maxRows := 1000
	if v := os.Getenv("MAX_QUERY_ROWS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("MAX_QUERY_ROWS must be a positive integer")
		}
		maxRows = n
	}

	timeoutSec := 30
	if v := os.Getenv("QUERY_TIMEOUT_SEC"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("QUERY_TIMEOUT_SEC must be a positive integer")
		}
		timeoutSec = n
	}

	rawPreviewBytes := int64(262144)
	if v := os.Getenv("RAW_PREVIEW_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("RAW_PREVIEW_BYTES must be a positive integer")
		}
		rawPreviewBytes = n
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	allowlist := make(map[string]struct{})
	if v := os.Getenv("BUCKET_ALLOWLIST"); v != "" {
		for _, b := range strings.Split(v, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				allowlist[b] = struct{}{}
			}
		}
	}

	return &Config{
		Port:            port,
		AWSRegion:       region,
		MaxQueryRows:    maxRows,
		QueryTimeoutSec: timeoutSec,
		RawPreviewBytes: rawPreviewBytes,
		BucketAllowlist: allowlist,
	}, nil
}

func (c *Config) BucketAllowed(bucket string) bool {
	if len(c.BucketAllowlist) == 0 {
		return true
	}
	_, ok := c.BucketAllowlist[bucket]
	return ok
}

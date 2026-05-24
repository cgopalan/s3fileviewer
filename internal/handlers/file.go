package handlers

import (
	"net/http"

	"github.com/cgopalan/s3fileviewer/internal/files"
)

func (h *Handler) File(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	key := r.URL.Query().Get("key")

	if err := h.validateBucket(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	sample, err := h.s3.GetObjectSample(r.Context(), bucket, key, 512)
	if err != nil {
		http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !files.IsAllowedExtension(key) {
		http.Error(w, "only .csv and .txt files are supported", http.StatusBadRequest)
		return
	}
	if !files.IsViewableTextFile(key, sample) {
		http.Error(w, "file appears to be binary and cannot be viewed", http.StatusBadRequest)
		return
	}

	uri := s3URI(bucket, key)
	browsePrefix := browsePrefixForKey(key)

	describeErr := h.duckdb.TryDescribe(bucket, key)
	if describeErr != nil {
		raw, rawErr := h.s3.GetObjectRange(r.Context(), bucket, key, h.cfg.RawPreviewBytes)
		if rawErr != nil {
			http.Error(w, "failed to load file preview: "+rawErr.Error(), http.StatusInternalServerError)
			return
		}
		h.render(w, "file_raw_content", map[string]any{
			"Title":        key,
			"Bucket":       bucket,
			"Key":          key,
			"S3URI":        uri,
			"BrowsePrefix": browsePrefix,
			"Content":      string(raw),
			"Truncated":    int64(len(raw)) >= h.cfg.RawPreviewBytes,
			"PreviewBytes": h.cfg.RawPreviewBytes,
			"DescribeErr":  describeErr.Error(),
		})
		return
	}

	h.render(w, "file_queryable_content", map[string]any{
		"Title":        key,
		"Bucket":       bucket,
		"Key":          key,
		"S3URI":        uri,
		"BrowsePrefix": browsePrefix,
		"MaxRows":      h.cfg.MaxQueryRows,
	})
}

func browsePrefixForKey(key string) string {
	if slash := lastSlash(key); slash >= 0 {
		return key[:slash+1]
	}
	return ""
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

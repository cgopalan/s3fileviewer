package handlers

import (
	"net/http"

	"github.com/cgopalan/s3fileviewer/internal/files"
)

type zipEntry struct {
	Name      string
	Queryable bool
}

func (h *Handler) File(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	key := r.URL.Query().Get("key")
	member := r.URL.Query().Get("member")

	if err := h.validateBucket(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}
	if !files.IsSupportedExtension(key) {
		http.Error(w, "unsupported file type (supported: .csv, .txt, .parquet, .zip, or extensionless as .csv)", http.StatusBadRequest)
		return
	}

	uri := s3URI(bucket, key)
	browsePrefix := browsePrefixForKey(key)

	if files.IsZipFile(key) && member == "" {
		names, err := h.s3.ListZipContents(r.Context(), bucket, key)
		if err != nil {
			http.Error(w, "failed to list zip contents: "+err.Error(), http.StatusInternalServerError)
			return
		}
		entries := make([]zipEntry, 0, len(names))
		for _, name := range names {
			entries = append(entries, zipEntry{
				Name:      name,
				Queryable: files.IsQueryableMember(name),
			})
		}
		h.render(w, "file_zip_content", map[string]any{
			"Title":        key,
			"Bucket":       bucket,
			"Key":          key,
			"S3URI":        uri,
			"BrowsePrefix": browsePrefix,
			"Entries":      entries,
		})
		return
	}

	if member != "" {
		if !files.IsQueryableMember(member) {
			http.Error(w, "unsupported file inside zip (supported: .csv, .txt, .parquet, or extensionless as .csv)", http.StatusBadRequest)
			return
		}
		displayKey := key + " → " + member
		h.render(w, "file_queryable_content", map[string]any{
			"Title":        displayKey,
			"Bucket":       bucket,
			"Key":          key,
			"Member":       member,
			"S3URI":        uri,
			"BrowsePrefix": browsePrefix,
			"MaxRows":      h.cfg.MaxQueryRows,
		})
		return
	} else if !files.IsBinaryQueryable(key) {
		sample, err := h.s3.GetObjectSample(r.Context(), bucket, key, 512)
		if err != nil {
			http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !files.IsViewableTextFile(key, sample) {
			http.Error(w, "file appears to be binary and cannot be viewed", http.StatusBadRequest)
			return
		}
	}

	describeErr := h.duckdb.TryDescribe(r.Context(), bucket, key, member)
	if describeErr != nil {
		if files.IsBinaryQueryable(key) || member != "" {
			http.Error(w, "failed to read file schema: "+describeErr.Error(), http.StatusInternalServerError)
			return
		}
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

	displayKey := key
	if member != "" {
		displayKey = key + " → " + member
	}

	h.render(w, "file_queryable_content", map[string]any{
		"Title":        displayKey,
		"Bucket":       bucket,
		"Key":          key,
		"Member":       member,
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

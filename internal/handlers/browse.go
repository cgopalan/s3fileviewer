package handlers

import (
	"net/http"

	"github.com/cgopalan/s3fileviewer/internal/files"
)

type browseObject struct {
	Key          string
	Name         string
	Size         int64
	LastModified string
	Viewable     bool
	IsBinary     bool
}

func (h *Handler) Browse(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	prefix := r.URL.Query().Get("prefix")

	if err := h.validateBucket(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.s3.ListObjects(r.Context(), bucket, prefix)
	if err != nil {
		http.Error(w, "failed to list bucket: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var objects []browseObject
	for _, obj := range result.Objects {
		viewable := false
		if files.IsSupportedExtension(obj.Key) {
			sample, _ := h.s3.GetObjectSample(r.Context(), bucket, obj.Key, 512)
			viewable = files.IsOpenable(obj.Key, sample)
		}
		objects = append(objects, browseObject{
			Key:          obj.Key,
			Name:         obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			Viewable:     viewable,
			IsBinary:     !viewable,
		})
	}

	h.render(w, "browse_content", map[string]any{
		"Title":         "Browse - " + bucket,
		"Bucket":        bucket,
		"Prefix":        prefix,
		"CurrentPrefix": prefix,
		"Prefixes":      result.Prefixes,
		"Objects":       objects,
		"Breadcrumbs":   buildBreadcrumbs(bucket, prefix),
	})
}

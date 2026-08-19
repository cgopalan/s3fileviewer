package handlers

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/cgopalan/s3fileviewer/internal/config"
	"github.com/cgopalan/s3fileviewer/internal/duckdb"
	"github.com/cgopalan/s3fileviewer/internal/files"
	s3client "github.com/cgopalan/s3fileviewer/internal/s3"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

type Handler struct {
	cfg    *config.Config
	s3     *s3client.Client
	duckdb *duckdb.Pool
	tmpl   *template.Template
}

func New(cfg *config.Config, s3 *s3client.Client, db *duckdb.Pool) (*Handler, error) {
	funcMap := template.FuncMap{
		"formatSize": formatSize,
		"urlquery":   url.QueryEscape,
		"trimPrefix": func(full, prefix string) string {
			return strings.TrimPrefix(full, prefix)
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/layout.html",
		"templates/home.html",
		"templates/browse.html",
		"templates/file_queryable.html",
		"templates/file_raw.html",
		"templates/file_zip.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{cfg: cfg, s3: s3, duckdb: db, tmpl: tmpl}, nil
}

func (h *Handler) StaticFS() http.FileSystem {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Home)
	r.Get("/browse", h.Browse)
	r.Get("/file", h.File)
	r.Get("/api/schema", h.Schema)
	r.Post("/api/query", h.Query)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(h.StaticFS())))
	return r
}

func (h *Handler) validateBucket(bucket string) error {
	if bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	if !bucketNameRe.MatchString(bucket) {
		return fmt.Errorf("invalid bucket name")
	}
	if !h.cfg.BucketAllowed(bucket) {
		return fmt.Errorf("bucket not allowed")
	}
	return nil
}

func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

type breadcrumb struct {
	Label string
	Prefix string
}

func buildBreadcrumbs(bucket, prefix string) []breadcrumb {
	crumbs := []breadcrumb{{Label: bucket, Prefix: ""}}
	if prefix == "" {
		return crumbs
	}
	trimmed := strings.TrimSuffix(prefix, "/")
	parts := strings.Split(trimmed, "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current += part + "/"
		crumbs = append(crumbs, breadcrumb{Label: part, Prefix: current})
	}
	return crumbs
}

func (h *Handler) render(w http.ResponseWriter, contentTemplate string, data map[string]any) {
	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, contentTemplate, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = template.HTML(buf.String())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func s3URI(bucket, key string) string {
	return files.S3URI(bucket, key)
}

package s3

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (c *Client) cachedZipPath(bucket, key string) (string, bool) {
	c.zipMu.Lock()
	defer c.zipMu.Unlock()
	if c.zipCacheKey != zipCacheKey(bucket, key) || c.zipCachePath == "" {
		return "", false
	}
	if _, err := os.Stat(c.zipCachePath); err != nil {
		return "", false
	}
	return c.zipCachePath, true
}

func (c *Client) removeZipCache() {
	if c.zipCachePath != "" {
		os.Remove(c.zipCachePath)
	}
	c.zipCacheKey = ""
	c.zipCachePath = ""
}

func (c *Client) ensureLocalZip(ctx context.Context, bucket, key string) (string, error) {
	cacheKey := zipCacheKey(bucket, key)

	c.zipMu.Lock()
	if c.zipCacheKey == cacheKey && c.zipCachePath != "" {
		if _, err := os.Stat(c.zipCachePath); err == nil {
			path := c.zipCachePath
			c.zipMu.Unlock()
			return path, nil
		}
	}
	c.zipMu.Unlock()

	path, err := c.downloadObjectToTemp(ctx, bucket, key, ".zip")
	if err != nil {
		return "", err
	}

	c.zipMu.Lock()
	if c.zipCachePath != "" && c.zipCachePath != path {
		os.Remove(c.zipCachePath)
	}
	c.zipCacheKey = cacheKey
	c.zipCachePath = path
	c.zipMu.Unlock()
	return path, nil
}

func (c *Client) prefetchZip(bucket, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	_, _ = c.ensureLocalZip(ctx, bucket, key)
}

func (c *Client) downloadObjectToTemp(ctx context.Context, bucket, key, suffix string) (string, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("get object: %w", err)
	}
	defer out.Body.Close()

	tmp, err := os.CreateTemp("", "s3fileviewer-zip-*"+suffix)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()

	if _, err := io.Copy(tmp, out.Body); err != nil {
		tmp.Close()
		os.Remove(path)
		return "", fmt.Errorf("download object: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return path, nil
}

type objectReaderAt struct {
	ctx    context.Context
	client *s3.Client
	bucket string
	key    string
	size   int64
}

func (r *objectReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= r.size {
		return 0, io.EOF
	}
	end := off + int64(len(p)) - 1
	if end >= r.size {
		end = r.size - 1
	}

	rangeHeader := fmt.Sprintf("bytes=%d-%d", off, end)
	out, err := r.client.GetObject(r.ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.key),
		Range:  aws.String(rangeHeader),
	})
	if err != nil {
		return 0, fmt.Errorf("get object range %s: %w", rangeHeader, err)
	}
	defer out.Body.Close()

	return io.ReadFull(out.Body, p[:end-off+1])
}

func (c *Client) HeadObjectSize(ctx context.Context, bucket, key string) (int64, error) {
	out, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("head object: %w", err)
	}
	if out.ContentLength == nil {
		return 0, fmt.Errorf("head object: missing content length")
	}
	return *out.ContentLength, nil
}

func (c *Client) openZipReaderRemote(ctx context.Context, bucket, key string) (*zip.Reader, error) {
	size, err := c.HeadObjectSize(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return nil, fmt.Errorf("zip archive is empty")
	}

	readerAt := &objectReaderAt{
		ctx:    ctx,
		client: c.client,
		bucket: bucket,
		key:    key,
		size:   size,
	}
	zr, err := zip.NewReader(readerAt, size)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	return zr, nil
}

func zipMemberNames(zr *zip.Reader) []string {
	var names []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func (c *Client) ListZipContents(ctx context.Context, bucket, key string) ([]string, error) {
	if path, ok := c.cachedZipPath(bucket, key); ok {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, fmt.Errorf("open cached zip: %w", err)
		}
		defer zr.Close()
		return zipMemberNames(&zr.Reader), nil
	}

	zr, err := c.openZipReaderRemote(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	names := zipMemberNames(zr)

	go c.prefetchZip(bucket, key)
	return names, nil
}

func tempSuffixForMember(member string) string {
	suffix := filepath.Ext(member)
	if suffix == "" {
		return ".csv"
	}
	return suffix
}

func extractMemberToTemp(zr *zip.Reader, member string) (string, error) {
	var entry *zip.File
	for _, f := range zr.File {
		if f.Name == member && !f.FileInfo().IsDir() {
			entry = f
			break
		}
	}
	if entry == nil {
		return "", fmt.Errorf("zip member %q not found", member)
	}

	rc, err := entry.Open()
	if err != nil {
		return "", fmt.Errorf("open zip member: %w", err)
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "s3fileviewer-*"+tempSuffixForMember(member))
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()

	if _, err := io.Copy(tmp, rc); err != nil {
		tmp.Close()
		os.Remove(path)
		return "", fmt.Errorf("extract zip member: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return path, nil
}

func (c *Client) ExtractZipMember(ctx context.Context, bucket, key, member string) (string, error) {
	zipPath, err := c.ensureLocalZip(ctx, bucket, key)
	if err != nil {
		return "", err
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip archive: %w", err)
	}
	defer zr.Close()

	return extractMemberToTemp(&zr.Reader, member)
}

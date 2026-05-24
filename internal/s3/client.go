package s3

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified string
}

type ListResult struct {
	Prefixes []string
	Objects  []ObjectInfo
}

type Client struct {
	client *s3.Client
	region string
}

func NewClient(ctx context.Context, region string) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Client{
		client: s3.NewFromConfig(cfg),
		region: region,
	}, nil
}

func (c *Client) ListObjects(ctx context.Context, bucket, prefix string) (*ListResult, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}

	var prefixes []string
	var objects []ObjectInfo

	paginator := s3.NewListObjectsV2Paginator(c.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, cp := range page.CommonPrefixes {
			if cp.Prefix != nil {
				prefixes = append(prefixes, *cp.Prefix)
			}
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key
			if key == prefix {
				continue
			}
			if strings.HasSuffix(key, "/") {
				prefixes = append(prefixes, key)
				continue
			}
			lastMod := ""
			if obj.LastModified != nil {
				lastMod = obj.LastModified.Format("2006-01-02 15:04:05")
			}
			size := int64(0)
			if obj.Size != nil {
				size = *obj.Size
			}
			objects = append(objects, ObjectInfo{
				Key:          key,
				Size:         size,
				LastModified: lastMod,
			})
		}
	}

	sort.Strings(prefixes)
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})

	return &ListResult{
		Prefixes: prefixes,
		Objects:  objects,
	}, nil
}

func (c *Client) GetObjectRange(ctx context.Context, bucket, key string, maxBytes int64) ([]byte, error) {
	rangeHeader := fmt.Sprintf("bytes=0-%d", maxBytes-1)
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeHeader),
	})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return data, nil
}

func (c *Client) GetObjectSample(ctx context.Context, bucket, key string, maxBytes int64) ([]byte, error) {
	return c.GetObjectRange(ctx, bucket, key, maxBytes)
}

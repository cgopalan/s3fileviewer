package files

import (
	"fmt"
	"strings"
)

type FileReader struct {
	ViewSQL     string
	DescribeSQL string
}

func S3URI(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

func DetectReader(source, key, member string) (FileReader, error) {
	if IsZipFile(key) && member == "" {
		return FileReader{}, fmt.Errorf("zip member is required")
	}

	ext := EffectiveExtension(key, member)
	loc := escapeSQLString(source)
	var expr string
	switch ext {
	case ".parquet":
		expr = fmt.Sprintf("read_parquet('%s')", loc)
	case ".csv", ".txt":
		expr = fmt.Sprintf("read_csv_auto('%s')", loc)
	default:
		return FileReader{}, fmt.Errorf("unsupported format %q", ext)
	}

	return FileReader{
		ViewSQL:     fmt.Sprintf("SELECT * FROM %s", expr),
		DescribeSQL: fmt.Sprintf("DESCRIBE SELECT * FROM %s", expr),
	}, nil
}

func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

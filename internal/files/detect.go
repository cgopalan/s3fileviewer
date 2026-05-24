package files

import "fmt"

type FileReader struct {
	ViewSQL     string
	DescribeSQL string
}

func S3URI(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

func DetectReader(bucket, key string) FileReader {
	uri := S3URI(bucket, key)
	expr := fmt.Sprintf("read_csv_auto('%s')", uri)
	return FileReader{
		ViewSQL:     fmt.Sprintf("SELECT * FROM %s", expr),
		DescribeSQL: fmt.Sprintf("DESCRIBE SELECT * FROM %s", expr),
	}
}

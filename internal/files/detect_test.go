package files

import "testing"

func TestDetectReaderCSV(t *testing.T) {
	source := "s3://my-bucket/path/data.csv"
	reader, err := DetectReader(source, "path/data.csv", "")
	if err != nil {
		t.Fatalf("DetectReader() error = %v", err)
	}
	want := "SELECT * FROM read_csv_auto('s3://my-bucket/path/data.csv')"
	if reader.ViewSQL != want {
		t.Errorf("ViewSQL = %q, want %q", reader.ViewSQL, want)
	}
}

func TestDetectReaderParquet(t *testing.T) {
	source := "s3://my-bucket/path/data.parquet"
	reader, err := DetectReader(source, "path/data.parquet", "")
	if err != nil {
		t.Fatalf("DetectReader() error = %v", err)
	}
	want := "SELECT * FROM read_parquet('s3://my-bucket/path/data.parquet')"
	if reader.ViewSQL != want {
		t.Errorf("ViewSQL = %q, want %q", reader.ViewSQL, want)
	}
}

func TestDetectReaderLocalZipMember(t *testing.T) {
	source := "/tmp/s3fileviewer-123.csv"
	reader, err := DetectReader(source, "archive.zip", "inner/data.csv")
	if err != nil {
		t.Fatalf("DetectReader() error = %v", err)
	}
	want := "SELECT * FROM read_csv_auto('/tmp/s3fileviewer-123.csv')"
	if reader.ViewSQL != want {
		t.Errorf("ViewSQL = %q, want %q", reader.ViewSQL, want)
	}
}

func TestDetectReaderLocalZipParquetMember(t *testing.T) {
	source := "/tmp/s3fileviewer-456.parquet"
	reader, err := DetectReader(source, "archive.zip", "inner/data.parquet")
	if err != nil {
		t.Fatalf("DetectReader() error = %v", err)
	}
	want := "SELECT * FROM read_parquet('/tmp/s3fileviewer-456.parquet')"
	if reader.ViewSQL != want {
		t.Errorf("ViewSQL = %q, want %q", reader.ViewSQL, want)
	}
}

func TestDetectReaderExtensionless(t *testing.T) {
	source := "s3://my-bucket/path/export"
	reader, err := DetectReader(source, "export", "")
	if err != nil {
		t.Fatalf("DetectReader() error = %v", err)
	}
	want := "SELECT * FROM read_csv_auto('s3://my-bucket/path/export')"
	if reader.ViewSQL != want {
		t.Errorf("ViewSQL = %q, want %q", reader.ViewSQL, want)
	}
}

func TestDetectReaderZipWithoutMember(t *testing.T) {
	_, err := DetectReader("s3://my-bucket/archive.zip", "archive.zip", "")
	if err == nil {
		t.Fatal("expected error for zip without member")
	}
}

func TestDetectReaderUnsupported(t *testing.T) {
	_, err := DetectReader("/tmp/readme.md", "archive.zip", "readme.md")
	if err == nil {
		t.Fatal("expected error for unsupported member extension")
	}
}

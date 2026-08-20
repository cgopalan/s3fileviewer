package s3

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

func TestZipMemberNames(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"b.csv", "a.parquet", "dir/"} {
		if name == "dir/" {
			_, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	data := buf.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	names := zipMemberNames(zr)
	if len(names) != 2 {
		t.Fatalf("len(names) = %d, want 2", len(names))
	}
	if names[0] != "a.parquet" || names[1] != "b.csv" {
		t.Fatalf("names = %v, want [a.parquet b.csv]", names)
	}
}

func TestExtractMemberToTemp(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("folder/data.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("a,b\n1,2")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	path, err := extractMemberToTemp(zr, "folder/data.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a,b\n1,2" {
		t.Fatalf("extracted = %q", string(data))
	}
}

func TestTempSuffixForMember(t *testing.T) {
	if got := tempSuffixForMember("data.csv"); got != ".csv" {
		t.Fatalf("got %q", got)
	}
	if got := tempSuffixForMember("export"); got != ".csv" {
		t.Fatalf("got %q", got)
	}
}

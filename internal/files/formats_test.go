package files

import "testing"

func TestIsSupportedExtension(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"data.csv", true},
		{"notes.txt", true},
		{"report.parquet", true},
		{"archive.zip", true},
		{"data", true},
		{"folder/export", true},
		{"image.png", false},
		{"DATA.CSV", true},
	}

	for _, tt := range tests {
		if got := IsSupportedExtension(tt.key); got != tt.want {
			t.Errorf("IsSupportedExtension(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestIsBinaryQueryable(t *testing.T) {
	if !IsBinaryQueryable("file.parquet") {
		t.Error("expected parquet to be binary queryable")
	}
	if !IsBinaryQueryable("file.zip") {
		t.Error("expected zip to be binary queryable")
	}
	if IsBinaryQueryable("file.csv") {
		t.Error("expected csv not to be binary queryable")
	}
}

func TestEffectiveExtension(t *testing.T) {
	if got := EffectiveExtension("archive.zip", "data.parquet"); got != ".parquet" {
		t.Errorf("EffectiveExtension() = %q, want .parquet", got)
	}
	if got := EffectiveExtension("data.csv", ""); got != ".csv" {
		t.Errorf("EffectiveExtension() = %q, want .csv", got)
	}
	if got := EffectiveExtension("export", ""); got != ".csv" {
		t.Errorf("EffectiveExtension() = %q, want .csv", got)
	}
	if got := EffectiveExtension("archive.zip", "inner/data"); got != ".csv" {
		t.Errorf("EffectiveExtension() = %q, want .csv", got)
	}
}

func TestIsOpenable(t *testing.T) {
	if !IsOpenable("data.parquet", nil) {
		t.Error("expected parquet to be openable")
	}
	if !IsOpenable("data.zip", nil) {
		t.Error("expected zip to be openable")
	}
	if !IsOpenable("data.csv", []byte("a,b\n1,2")) {
		t.Error("expected valid csv to be openable")
	}
	if !IsOpenable("export", []byte("a,b\n1,2")) {
		t.Error("expected extensionless file to be openable as csv")
	}
	if IsOpenable("data.csv", []byte{0x00, 0x01}) {
		t.Error("expected binary csv sample not to be openable")
	}
	if IsOpenable("image.png", nil) {
		t.Error("expected png not to be openable")
	}
}

func TestIsQueryableMember(t *testing.T) {
	for _, name := range []string{"a.csv", "b.txt", "c.parquet", "data"} {
		if !IsQueryableMember(name) {
			t.Errorf("expected %q to be queryable", name)
		}
	}
	if IsQueryableMember("readme.md") {
		t.Error("expected md not to be queryable")
	}
}

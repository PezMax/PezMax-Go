package fileurl

import (
	"errors"
	"strings"
	"testing"
)

func newTestBuilder(t *testing.T) *Builder {
	t.Helper()
	builder, err := NewBuilder("ptmj", "http://files.example.com:29000/")
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	return builder
}

func TestBuilderComposesPublicURLs(t *testing.T) {
	builder := newTestBuilder(t)
	cases := []struct{ key, want string }{
		{"科目/学校/2024/试卷 final.docx", "http://files.example.com:29000/ptmj/%E7%A7%91%E7%9B%AE/%E5%AD%A6%E6%A0%A1/2024/%E8%AF%95%E5%8D%B7%20final.docx"},
		{"/leading/slash.pdf", "http://files.example.com:29000/ptmj/leading/slash.pdf"},
		{"../escape/normalized.txt", "http://files.example.com:29000/ptmj/escape/normalized.txt"},
	}
	for _, item := range cases {
		got, err := builder.ObjectURL(item.key)
		if err != nil {
			t.Fatalf("ObjectURL(%q): %v", item.key, err)
		}
		if got != item.want {
			t.Fatalf("ObjectURL(%q) = %q, want %q", item.key, got, item.want)
		}
	}
	if builder.MustObjectURL("bad\x00key") != "" {
		t.Fatal("invalid key should render empty via MustObjectURL")
	}
	if builder.Bucket() != "ptmj" {
		t.Fatalf("Bucket = %q", builder.Bucket())
	}
}

func TestNewBuilderValidatesInput(t *testing.T) {
	if _, err := NewBuilder("ptmj", ""); err == nil {
		t.Fatal("empty public base should fail")
	}
	if _, err := NewBuilder("ptmj", "::::"); err == nil {
		t.Fatal("invalid public base should fail")
	}
	builder, err := NewBuilder("/ptmj/", "http://files.example.com:29000/base")
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if !strings.HasPrefix(builder.MustObjectURL("a.pdf"), "http://files.example.com:29000/base/ptmj/a.pdf") {
		t.Fatalf("prefix path not preserved: %s", builder.MustObjectURL("a.pdf"))
	}
}

func TestFromEnvRequiresPublicBase(t *testing.T) {
	if _, err := FromEnv(func(key, fallback string) string {
		if key == "KADMIN_MINIO_BUCKET" {
			return "ptmj"
		}
		return fallback
	}); err == nil {
		t.Fatal("FromEnv without KADMIN_MINIO_PUBLIC_BASE should fail")
	}
	builder, err := FromEnv(func(key, fallback string) string {
		if key == "KADMIN_MINIO_BUCKET" {
			return "ptmj"
		}
		if key == "KADMIN_MINIO_PUBLIC_BASE" {
			return "http://127.0.0.1:29000"
		}
		return fallback
	})
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if builder.Bucket() != "ptmj" {
		t.Fatalf("Bucket = %q", builder.Bucket())
	}
}

func TestParseAcceptsAnyStoredHostForm(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		bucket string
		key    string
	}{
		{"legacy internal host", "http://minio:9000/ptmj/%E7%A7%91%E7%9B%AE/exam.pdf", "ptmj", "科目/exam.pdf"},
		{"legacy loopback", "http://127.0.0.1:9000/ptmj/a b.pdf", "ptmj", "a b.pdf"},
		{"current public form", "https://files.example.com/ptmj/math/exam.pdf?x=1", "ptmj", "math/exam.pdf"},
		{"minio scheme", "minio://ptmj/科目/exam.pdf", "ptmj", "科目/exam.pdf"},
		{"minio scheme encoded", "minio://ptmj/%E7%A7%91%E7%9B%AE/exam.pdf", "ptmj", "科目/exam.pdf"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			bucket, key, err := Parse(item.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", item.in, err)
			}
			if bucket != item.bucket || key != item.key {
				t.Fatalf("Parse(%q) = (%q, %q), want (%q, %q)", item.in, bucket, key, item.bucket, item.key)
			}
		})
	}
}

func TestParseRejectsUnsupportedValues(t *testing.T) {
	for _, value := range []string{"", "   ", "/uploads/relative.pdf", "data:image/png;base64,AAAA", "http://host/only-bucket"} {
		if _, _, err := Parse(value); !errors.Is(err, ErrUnsupportedURL) {
			t.Fatalf("Parse(%q) error = %v, want ErrUnsupportedURL", value, err)
		}
	}
}

// TestWriteThenParseRoundTrip pins the end-to-end contract: whatever the
// builder writes must parse back to the same bucket/object pair.
func TestWriteThenParseRoundTrip(t *testing.T) {
	builder := newTestBuilder(t)
	const key = "科目/学校/2024/试卷 final.docx"
	stored, err := builder.ObjectURL(key)
	if err != nil {
		t.Fatalf("ObjectURL: %v", err)
	}
	bucket, parsedKey, err := Parse(stored)
	if err != nil {
		t.Fatalf("Parse(%q): %v", stored, err)
	}
	if bucket != builder.Bucket() || parsedKey != key {
		t.Fatalf("round trip = (%q, %q), want (%q, %q)", bucket, parsedKey, builder.Bucket(), key)
	}
}

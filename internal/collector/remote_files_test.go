package collector

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTar(t *testing.T) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	content := []byte("test-data")
	if err := writer.WriteHeader(&tar.Header{Name: "dir/file.txt", Mode: 0o640, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	if err := extractTar(bytes.NewReader(buffer.Bytes()), destination); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(filepath.Join(destination, "dir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(content) {
		t.Fatalf("content = %q", result)
	}
}

func TestExtractTarRejectsParentTraversal(t *testing.T) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := writer.WriteHeader(&tar.Header{Name: "../outside", Mode: 0o600, Size: 0, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractTar(bytes.NewReader(buffer.Bytes()), t.TempDir()); err == nil {
		t.Fatal("extractTar() accepted a path outside the destination")
	}
}

func TestShellQuote(t *testing.T) {
	if value := shellQuote("a'b"); value != `'a'"'"'b'` {
		t.Fatalf("shellQuote() = %q", value)
	}
}

func TestExtractTarReplacesTargetSymlink(t *testing.T) {
	destination := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(destination, "file.txt")); err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	content := []byte("inside")
	if err := writer.WriteHeader(&tar.Header{Name: "file.txt", Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractTar(bytes.NewReader(buffer.Bytes()), destination); err != nil {
		t.Fatal(err)
	}
	outsideContent, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(outsideContent) != "outside" {
		t.Fatalf("outside content = %q", outsideContent)
	}
	insideContent, err := os.ReadFile(filepath.Join(destination, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(insideContent) != "inside" {
		t.Fatalf("inside content = %q", insideContent)
	}
}

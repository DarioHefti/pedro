package documentparser

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestNativeParsePlainText(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sample.txt")
	content := "line one\nline two\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := (Native{}).Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != "text" {
		t.Fatalf("format: got %q", res.Format)
	}
	if len(res.Lines) != 2 || res.Lines[0] != "line one" {
		t.Fatalf("lines: %#v", res.Lines)
	}
}

func TestNativeUnsupportedImage(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.png")
	if err := os.WriteFile(p, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Native{}).Parse(p)
	if err == nil {
		t.Fatal("expected error for png")
	}
	var eu ErrUnsupported
	if !errors.As(err, &eu) {
		t.Fatalf("expected ErrUnsupported, got %T %v", err, err)
	}
}

func TestExtractExcelAllSheets(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "book.xlsx")
	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A1", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(p); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := ExtractExcelAllSheets(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != "excel" {
		t.Fatalf("format %q", res.Format)
	}
	found := false
	for _, line := range res.Lines {
		if line == `1: hello` {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing row, lines=%v", res.Lines)
	}
}

func TestNativeParseDocxMinimal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "minimal.docx")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<?xml version="1.0"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body><w:p><w:r><w:t>HelloDocx</w:t></w:r></w:p></w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := (Native{}).Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != "docx" {
		t.Fatalf("format %q", res.Format)
	}
	found := false
	for _, line := range res.Lines {
		if line == "HelloDocx" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected HelloDocx in %#v", res.Lines)
	}
}

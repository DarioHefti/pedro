package documentparser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func extractDocx(path string) (*Result, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var docXML []byte
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			docXML, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if len(docXML) == 0 {
		return nil, fmt.Errorf("docx missing word/document.xml")
	}

	text := extractWordTextFromDocumentXML(docXML)
	lines := splitNonEmptyLines(text)
	if len(lines) == 0 {
		lines = []string{"(No text content found in DOCX)"}
	}

	return &Result{
		Format:     "docx",
		FileSizeMB: fileSizeMB,
		Lines:      lines,
	}, nil
}

func extractWordTextFromDocumentXML(data []byte) string {
	// Fast path: collect w:t text runs (Office Open XML).
	re := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	var b strings.Builder
	for _, m := range matches {
		if len(m) > 1 {
			b.WriteString(m[1])
			b.WriteByte(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

func extractPptx(path string) (*Result, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var slideNames []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideNames = append(slideNames, f.Name)
		}
	}
	sort.Slice(slideNames, func(i, j int) bool {
		numI := slideNumber(slideNames[i])
		numJ := slideNumber(slideNames[j])
		return numI < numJ
	})

	var lines []string
	if len(slideNames) == 0 {
		return &Result{
			Format:     "pptx",
			FileSizeMB: fileSizeMB,
			Lines:      []string{"(No slides found in PPTX)"},
		}, nil
	}

	for _, name := range slideNames {
		var zf *zip.File
		for _, f := range r.File {
			if f.Name == name {
				zf = f
				break
			}
		}
		if zf == nil {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		slideXML, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}
		n := slideNumber(name)
		lines = append(lines, fmt.Sprintf("--- Slide %d ---", n))
		slideText := extractPptTextFromSlideXML(slideXML)
		for _, line := range splitNonEmptyLines(slideText) {
			lines = append(lines, line)
		}
	}

	return &Result{
		Format:     "pptx",
		FileSizeMB: fileSizeMB,
		Lines:      lines,
	}, nil
}

func slideNumber(name string) int {
	base := filepath.Base(name)
	// slide2.xml -> 2
	re := regexp.MustCompile(`slide(\d+)\.xml`)
	m := re.FindStringSubmatch(base)
	if len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func extractPptTextFromSlideXML(data []byte) string {
	re := regexp.MustCompile(`<a:t>([^<]*)</a:t>`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	var b strings.Builder
	for _, m := range matches {
		if len(m) > 1 {
			b.WriteString(m[1])
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func extractOdt(path string) (*Result, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var content []byte
	for _, f := range r.File {
		if f.Name == "content.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			content, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("odt missing content.xml")
	}

	text := extractOdtText(content)
	lines := splitNonEmptyLines(text)
	if len(lines) == 0 {
		lines = []string{"(No text content found in ODT)"}
	}

	return &Result{
		Format:     "odt",
		FileSizeMB: fileSizeMB,
		Lines:      lines,
	}, nil
}

// extractOdtText uses a lightweight XML token walk (OpenDocument text namespace).
func extractOdtText(data []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return strings.TrimSpace(out.String())
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "line-break" {
				out.WriteByte('\n')
			}
		case xml.CharData:
			s := strings.TrimSpace(string(el))
			if s != "" {
				if out.Len() > 0 {
					out.WriteByte(' ')
				}
				out.WriteString(s)
			}
		}
	}
	return strings.TrimSpace(out.String())
}

func splitNonEmptyLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

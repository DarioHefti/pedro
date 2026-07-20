package webclaw

import "errors"

var (
	ErrNoContent  = errors.New("webclaw: no content found")
	ErrInvalidURL = errors.New("webclaw: invalid URL")
)

// ExtractionOptions controls what gets extracted.
type ExtractionOptions struct {
	IncludeSelectors []string
	ExcludeSelectors []string
	OnlyMainContent  bool
	IncludeRawHTML   bool
}

func DefaultOptions() *ExtractionOptions {
	return &ExtractionOptions{}
}

// ExtractionResult is the full output of extraction.
type ExtractionResult struct {
	Metadata       Metadata            `json:"metadata"`
	Content        Content             `json:"content"`
	DomainData     *DomainData         `json:"domain_data,omitempty"`
	StructuredData []map[string]any    `json:"structured_data,omitempty"`
}

// Content holds the extracted text and assets.
type Content struct {
	Markdown   string      `json:"markdown"`
	PlainText  string      `json:"plain_text"`
	Links      []Link      `json:"links"`
	Images     []Image     `json:"images"`
	CodeBlocks []CodeBlock `json:"code_blocks"`
	RawHTML    *string     `json:"raw_html,omitempty"`
}

// Metadata from <head> and page analysis.
type Metadata struct {
	Title       *string           `json:"title,omitempty"`
	Description *string           `json:"description,omitempty"`
	Author      *string           `json:"author,omitempty"`
	Language    *string           `json:"language,omitempty"`
	WordCount   int               `json:"word_count"`
	OpenGraph   map[string]string `json:"open_graph,omitempty"`
}

// Link represents an extracted hyperlink.
type Link struct {
	Text string `json:"text"`
	Href string `json:"href"`
}

// Image represents an extracted image.
type Image struct {
	Alt string `json:"alt"`
	Src string `json:"src"`
}

// CodeBlock represents an extracted fenced code block.
type CodeBlock struct {
	Language *string `json:"language,omitempty"`
	Code     string  `json:"code"`
}

// DomainData holds domain-type detection results.
type DomainData struct {
	DomainType string `json:"domain_type"`
}

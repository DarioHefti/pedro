package webclaw

import (
	"strings"
	"testing"
)

func TestExtractBasic(t *testing.T) {
	html := `<html lang="en">
<head><title>Test Page</title><meta name="description" content="A test"></head>
<body>
<nav><a href="/">Home</a></nav>
<article>
<h1>Hello World</h1>
<p>This is the main content of the page with enough text to be scored properly by the readability algorithm.</p>
<p>Second paragraph with more content to increase the word count and make the scoring work correctly.</p>
</article>
<footer>Copyright 2025</footer>
</body></html>`

	result, err := Extract(html, strPtr("https://example.com"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata.Title == nil || *result.Metadata.Title != "Test Page" {
		t.Errorf("title = %v, want 'Test Page'", result.Metadata.Title)
	}
	if !strings.Contains(result.Content.Markdown, "Hello World") {
		t.Error("markdown should contain heading")
	}
	if !strings.Contains(result.Content.Markdown, "main content") {
		t.Error("markdown should contain article content")
	}
	if strings.Contains(result.Content.Markdown, "Copyright") {
		t.Error("markdown should not contain footer")
	}
}

func TestExtractEmpty(t *testing.T) {
	_, err := Extract("", nil, nil)
	if err != ErrNoContent {
		t.Errorf("expected ErrNoContent, got %v", err)
	}
}

func TestExtractExcludeSelectors(t *testing.T) {
	html := `<html><body>
<nav>Navigation stuff</nav>
<article><h1>Title</h1><p>Real content here with enough words for scoring.</p></article>
<footer>Footer stuff</footer>
</body></html>`

	opts := &ExtractionOptions{
		ExcludeSelectors: []string{"nav", "footer"},
	}
	result, err := Extract(html, nil, opts)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result.Content.Markdown, "Navigation") {
		t.Error("nav should be excluded")
	}
	if strings.Contains(result.Content.Markdown, "Footer") {
		t.Error("footer should be excluded")
	}
	if !strings.Contains(result.Content.Markdown, "Real content") {
		t.Error("article content should be present")
	}
}

func TestExtractIncludeSelectors(t *testing.T) {
	html := `<html><body>
<nav>Navigation</nav>
<article><h1>Title</h1><p>Real content here.</p></article>
<div class="sidebar">Sidebar junk</div>
</body></html>`

	opts := &ExtractionOptions{
		IncludeSelectors: []string{"article"},
	}
	result, err := Extract(html, nil, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "Title") {
		t.Error("article title should be present")
	}
	if strings.Contains(result.Content.Markdown, "Navigation") {
		t.Error("nav should not be included")
	}
	if strings.Contains(result.Content.Markdown, "Sidebar") {
		t.Error("sidebar should not be included")
	}
}

func TestExtractOnlyMainContent(t *testing.T) {
	html := `<html><body>
<nav>Navigation</nav>
<div class="hero"><h1>Big Hero</h1></div>
<article><h2>Article Title</h2><p>Article content that is long enough to be detected as main content for scoring.</p></article>
<div class="sidebar">Sidebar</div>
</body></html>`

	opts := &ExtractionOptions{
		OnlyMainContent: true,
	}
	result, err := Extract(html, nil, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "Article Title") {
		t.Error("article content should be present")
	}
}

func TestMetadata(t *testing.T) {
	html := `<html lang="en">
<head>
<title>My Page</title>
<meta name="description" content="Page description">
<meta name="author" content="John Doe">
<meta property="og:title" content="OG Title">
</head>
<body><article><p>Content here with enough words.</p></article></body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata.Title == nil || *result.Metadata.Title != "My Page" {
		t.Errorf("title = %v", result.Metadata.Title)
	}
	if result.Metadata.Description == nil || *result.Metadata.Description != "Page description" {
		t.Errorf("description = %v", result.Metadata.Description)
	}
	if result.Metadata.Author == nil || *result.Metadata.Author != "John Doe" {
		t.Errorf("author = %v", result.Metadata.Author)
	}
	if result.Metadata.Language == nil || *result.Metadata.Language != "en" {
		t.Errorf("language = %v", result.Metadata.Language)
	}
	if result.Metadata.OpenGraph["title"] != "OG Title" {
		t.Errorf("og:title = %v", result.Metadata.OpenGraph["title"])
	}
}

func TestCodeBlock(t *testing.T) {
	html := `<html><body>
<article>
<p>Here is some code:</p>
<pre><code class="language-go">func main() {
	fmt.Println("hello")
}</code></pre>
</article>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Content.CodeBlocks) == 0 {
		t.Fatal("expected code blocks")
	}
	if result.Content.CodeBlocks[0].Language == nil || *result.Content.CodeBlocks[0].Language != "go" {
		t.Errorf("language = %v", result.Content.CodeBlocks[0].Language)
	}
	if !strings.Contains(result.Content.Markdown, "```go") {
		t.Error("markdown should contain go code fence")
	}
}

func TestLinks(t *testing.T) {
	html := `<html><body>
<article>
<p>Visit <a href="https://example.com">Example</a> for more info.</p>
<p><a href="/about">About us</a> page.</p>
</article>
</body></html>`

	result, err := Extract(html, strPtr("https://site.com"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Content.Links) < 2 {
		t.Errorf("expected at least 2 links, got %d", len(result.Content.Links))
	}
}

func TestImages(t *testing.T) {
	html := `<html><body>
<article>
<p><img src="https://example.com/photo.jpg" alt="A photo"></p>
</article>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Content.Images) == 0 {
		t.Fatal("expected images")
	}
	if result.Content.Images[0].Alt != "A photo" {
		t.Errorf("alt = %v", result.Content.Images[0].Alt)
	}
}

func TestWordCount(t *testing.T) {
	html := `<html><body>
<article>
<h1>Title</h1>
<p>This is a paragraph with several words that should be counted properly.</p>
<p>Another paragraph with more words to increase the total word count.</p>
</article>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata.WordCount < 10 {
		t.Errorf("word count = %d, want >= 10", result.Metadata.WordCount)
	}
}

func TestNoiseStripping(t *testing.T) {
	html := `<html><body>
<div class="cookie-banner"><p>We use cookies</p></div>
<div class="sidebar"><p>Sidebar content</p></div>
<article>
<h1>Article</h1>
<p>Real article content that should be extracted and kept in the output.</p>
</article>
<div role="navigation"><a href="/">Home</a></div>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result.Content.Markdown, "cookies") {
		t.Error("cookie banner should be stripped")
	}
	if strings.Contains(result.Content.Markdown, "Sidebar") {
		t.Error("sidebar should be stripped")
	}
	if !strings.Contains(result.Content.Markdown, "Article") {
		t.Error("article should be present")
	}
}

func TestTable(t *testing.T) {
	html := `<html><body>
<article>
<table>
<thead><tr><th>Name</th><th>Age</th></tr></thead>
<tbody><tr><td>Alice</td><td>30</td></tr></tbody>
</table>
</article>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "| Name | Age |") {
		t.Error("markdown should contain table header")
	}
	if !strings.Contains(result.Content.Markdown, "| Alice | 30 |") {
		t.Error("markdown should contain table row")
	}
}

func TestBlockquote(t *testing.T) {
	html := `<html><body>
<article>
<blockquote><p>A wise quote from someone important.</p></blockquote>
</article>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "> A wise quote") {
		t.Error("markdown should contain blockquote")
	}
}

func TestList(t *testing.T) {
	html := `<html><body>
<article>
<ul>
<li>First item</li>
<li>Second item</li>
</ul>
</article>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "- First item") {
		t.Error("markdown should contain first list item")
	}
	if !strings.Contains(result.Content.Markdown, "- Second item") {
		t.Error("markdown should contain second list item")
	}
}

func TestPlainText(t *testing.T) {
	html := `<html><body>
<article>
<h1>Title</h1>
<p>Hello <strong>world</strong> and <em>stuff</em>.</p>
</article>
</body></html>`

	result, err := Extract(html, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result.Content.PlainText, "**") {
		t.Error("plain text should not contain bold markers")
	}
	if strings.Contains(result.Content.PlainText, "*") {
		t.Error("plain text should not contain italic markers")
	}
}

func TestIncludeRawHTML(t *testing.T) {
	html := `<html><body>
<article><h1>Title</h1><p>Content here.</p></article>
</body></html>`

	opts := &ExtractionOptions{
		IncludeRawHTML: true,
	}
	result, err := Extract(html, nil, opts)
	if err != nil {
		t.Fatal(err)
	}

	if result.Content.RawHTML == nil {
		t.Error("raw_html should be populated")
	}
	if result.Content.RawHTML != nil && !strings.Contains(*result.Content.RawHTML, "<article>") {
		t.Error("raw_html should contain article tag")
	}
}

func TestRelativeURLResolution(t *testing.T) {
	html := `<html><body>
<article>
<p><a href="/about">About</a></p>
<p><img src="/image.png" alt="Image"></p>
</article>
</body></html>`

	result, err := Extract(html, strPtr("https://example.com/page"), nil)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, link := range result.Content.Links {
		if link.Href == "https://example.com/about" {
			found = true
			break
		}
	}
	if !found {
		t.Error("relative URL should be resolved")
	}
}

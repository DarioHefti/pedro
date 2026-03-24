package tools

import (
	"strings"
	"testing"
)

func TestLooksLikeBlockedPage(t *testing.T) {
	if !looksLikeBlockedPage("<html>please complete the captcha</html>") {
		t.Fatal("expected captcha body to be blocked")
	}
	if looksLikeBlockedPage("normal article text without challenges") {
		t.Fatal("normal text should not match")
	}
}

func TestLooksLikeJSRequired(t *testing.T) {
	if !looksLikeJSRequired("JavaScript is required to view this page") {
		t.Fatal("expected js-required signal")
	}
	if looksLikeJSRequired("full article paragraph") {
		t.Fatal("plain text should not match")
	}
}

func TestNeedsJSRendering(t *testing.T) {
	if !needsJSRendering("https://www.fedlex.admin.ch/eli/cc/1999/123/de") {
		t.Fatal("fedlex should use JS path")
	}
	if needsJSRendering("https://example.com/article") {
		t.Fatal("generic URL should not force JS")
	}
}

func TestLooksLikeHTML(t *testing.T) {
	if !looksLikeHTML("<!DOCTYPE html><html>") {
		t.Fatal("doctype prefix")
	}
	if !looksLikeHTML("  <body>") {
		t.Fatal("body tag")
	}
	if looksLikeHTML("plain text") {
		t.Fatal("plain text is not HTML")
	}
}

func TestConvertToMarkdown(t *testing.T) {
	out, err := convertToMarkdown("<p>Hello <strong>world</strong></p>")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out), "hello") {
		t.Fatalf("expected hello in %q", out)
	}
}

func TestFetchURLToolExecuteInvalidJSON(t *testing.T) {
	var f FetchURLTool
	s, err := f.Execute("{not json")
	if err != nil {
		t.Fatalf("Execute returns string errors to model: %v", err)
	}
	if !strings.Contains(s, "Error parsing arguments") {
		t.Fatalf("unexpected result: %q", s)
	}
}

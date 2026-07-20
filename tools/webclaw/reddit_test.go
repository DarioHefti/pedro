package webclaw

import (
	"strings"
	"testing"
)

func TestIsRedditURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.reddit.com/r/rust/comments/abc/x/", true},
		{"https://old.reddit.com/r/rust/comments/abc/x/", true},
		{"https://reddit.com/r/rust/comments/abc/x/", true},
		{"https://np.reddit.com/r/rust/comments/abc/x/", true},
		{"https://new.reddit.com/r/rust/comments/abc/x/", true},
		{"https://example.com", false},
		{"https://github.com/test", false},
	}
	for _, tt := range tests {
		if got := isRedditURL(tt.url); got != tt.want {
			t.Errorf("isRedditURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestRedditExtraction(t *testing.T) {
	html := `<html><body>
<div id="siteTable">
<div class="thing link self" data-fullname="t3_abc123" data-author="testuser" data-subreddit="programming" data-score="42" data-comments-count="5" data-permalink="/r/programming/comments/abc123/test_post/" data-url="https://example.com" data-domain="self.programming">
<a class="title" href="/r/programming/comments/abc123/test_post/">Test Post Title</a>
<div class="entry">
<div class="expando">
<div class="usertext-body">
<div class="md"><p>This is the self text body with enough content to be extracted properly.</p></div>
</div>
</div>
</div>
</div>
</div>

<div class="commentarea">
<div class="sitetable nestedlisting">
<div class="comment thing" data-fullname="t1_comment1" data-author="commenter1" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"><p>First comment with some content.</p></div>
</div>
</div>
</div>
<div class="comment thing" data-fullname="t1_comment2" data-author="commenter2" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"><p>Second comment with more content.</p></div>
</div>
</div>
</div>
</div>
</div>
</body></html>`

	result, err := Extract(html, strPtr("https://www.reddit.com/r/programming/comments/abc123/test_post/"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check metadata
	if result.Metadata.Title == nil || *result.Metadata.Title != "Test Post Title" {
		t.Errorf("title = %v, want 'Test Post Title'", result.Metadata.Title)
	}
	if result.Metadata.Author == nil || *result.Metadata.Author != "testuser" {
		t.Errorf("author = %v, want 'testuser'", result.Metadata.Author)
	}

	// Check markdown
	if !strings.Contains(result.Content.Markdown, "# Test Post Title") {
		t.Error("markdown should contain post title as heading")
	}
	if !strings.Contains(result.Content.Markdown, "u/testuser") {
		t.Error("markdown should contain post author")
	}
	if !strings.Contains(result.Content.Markdown, "r/programming") {
		t.Error("markdown should contain subreddit")
	}
	if !strings.Contains(result.Content.Markdown, "42 pts") {
		t.Error("markdown should contain score")
	}
	if !strings.Contains(result.Content.Markdown, "self text body") {
		t.Error("markdown should contain self text")
	}

	// Check comments
	if !strings.Contains(result.Content.Markdown, "## Comments") {
		t.Error("markdown should contain comments section")
	}
	if !strings.Contains(result.Content.Markdown, "First comment") {
		t.Error("markdown should contain first comment")
	}
	if !strings.Contains(result.Content.Markdown, "Second comment") {
		t.Error("markdown should contain second comment")
	}
	if !strings.Contains(result.Content.Markdown, "u/commenter1") {
		t.Error("markdown should contain first commenter")
	}
}

func TestRedditNonCommentURL(t *testing.T) {
	// Non-comment URL should not trigger Reddit extraction
	html := `<html><body><article><p>Some content</p></article></body></html>`
	result, err := Extract(html, strPtr("https://www.reddit.com/r/programming/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should use generic extraction, not Reddit
	if strings.Contains(result.Content.Markdown, "## Comments") {
		t.Error("non-comment URL should not have Reddit comments section")
	}
}

func TestRedditNonRedditURL(t *testing.T) {
	// Non-Reddit URL should use generic extraction
	html := `<html><body><article><h1>Article</h1><p>Content here.</p></article></body></html>`
	result, err := Extract(html, strPtr("https://example.com/article"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Content.Markdown, "## Comments") {
		t.Error("non-Reddit URL should not have Reddit comments section")
	}
}

func TestRedditDeletedComment(t *testing.T) {
	html := `<html><body>
<div id="siteTable">
<div class="thing link self" data-fullname="t3_abc" data-author="op" data-subreddit="test" data-score="10" data-comments-count="1" data-permalink="/r/test/comments/abc/test/">
<a class="title" href="/r/test/comments/abc/test/">Test</a>
</div>
</div>
<div class="commentarea">
<div class="sitetable nestedlisting">
<div class="comment thing deleted" data-fullname="t1_del" data-author="" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"></div>
</div>
</div>
</div>
</div>
</div>
</body></html>`

	result, err := Extract(html, strPtr("https://www.reddit.com/r/test/comments/abc/test/"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "[removed]") {
		t.Error("deleted comment should show [removed]")
	}
}

func TestRedditScoreHidden(t *testing.T) {
	html := `<html><body>
<div id="siteTable">
<div class="thing link self" data-fullname="t3_abc" data-author="op" data-subreddit="test" data-score="5" data-comments-count="1" data-permalink="/r/test/comments/abc/test/">
<a class="title" href="/r/test/comments/abc/test/">Test</a>
</div>
</div>
<div class="commentarea">
<div class="sitetable nestedlisting">
<div class="comment thing" data-fullname="t1_fresh" data-author="newuser" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"><p>Fresh comment.</p></div>
</div>
</div>
</div>
</div>
</div>
</body></html>`

	result, err := Extract(html, strPtr("https://www.reddit.com/r/test/comments/abc/test/"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Fresh comment without score span should show "score hidden"
	if !strings.Contains(result.Content.Markdown, "score hidden") {
		t.Error("fresh comment should show 'score hidden'")
	}
}

func TestRedditNestedComments(t *testing.T) {
	html := `<html><body>
<div id="siteTable">
<div class="thing link self" data-fullname="t3_abc" data-author="op" data-subreddit="test" data-score="10" data-comments-count="2" data-permalink="/r/test/comments/abc/test/">
<a class="title" href="/r/test/comments/abc/test/">Test</a>
</div>
</div>
<div class="commentarea">
<div class="sitetable nestedlisting">
<div class="comment thing" data-fullname="t1_parent" data-author="parent" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"><p>Parent comment.</p></div>
</div>
</div>
<div class="child">
<div class="sitetable">
<div class="comment thing" data-fullname="t1_child" data-author="child" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"><p>Child reply.</p></div>
</div>
</div>
</div>
</div>
</div>
</div>
</div>
</div>
</body></html>`

	result, err := Extract(html, strPtr("https://www.reddit.com/r/test/comments/abc/test/"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "Parent comment") {
		t.Error("should contain parent comment")
	}
	if !strings.Contains(result.Content.Markdown, "Child reply") {
		t.Error("should contain child reply")
	}
	// Nested comment should be quoted
	if !strings.Contains(result.Content.Markdown, "> **u/child**") {
		t.Error("nested comment should be quoted")
	}
}

func TestRedditExternalLinkPost(t *testing.T) {
	html := `<html><body>
<div id="siteTable">
<div class="thing link" data-fullname="t3_abc" data-author="poster" data-subreddit="programming" data-score="100" data-comments-count="20" data-permalink="/r/programming/comments/abc/link_post/" data-url="https://example.com/article" data-domain="example.com">
<a class="title" href="/r/programming/comments/abc/link_post/">Interesting Article</a>
</div>
</div>
</body></html>`

	result, err := Extract(html, strPtr("https://www.reddit.com/r/programming/comments/abc/link_post/"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Content.Markdown, "# Interesting Article") {
		t.Error("should contain post title")
	}
	if !strings.Contains(result.Content.Markdown, "[Link](https://example.com/article)") {
		t.Error("should contain link to external article")
	}
}

func TestRedditOPDetection(t *testing.T) {
	html := `<html><body>
<div id="siteTable">
<div class="thing link self" data-fullname="t3_abc" data-author="op_user" data-subreddit="test" data-score="10" data-comments-count="2" data-permalink="/r/test/comments/abc/test/">
<a class="title" href="/r/test/comments/abc/test/">Test</a>
</div>
</div>
<div class="commentarea">
<div class="sitetable nestedlisting">
<div class="comment thing" data-fullname="t1_c1" data-author="op_user" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"><p>OP reply.</p></div>
</div>
</div>
</div>
<div class="comment thing" data-fullname="t1_c2" data-author="other_user" data-type="comment">
<div class="entry">
<div class="usertext-body">
<div class="md"><p>Regular comment.</p></div>
</div>
</div>
</div>
</div>
</div>
</body></html>`

	result, err := Extract(html, strPtr("https://www.reddit.com/r/test/comments/abc/test/"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// OP should be marked
	if !strings.Contains(result.Content.Markdown, "u/op_user [OP]") {
		t.Error("OP should be marked with [OP]")
	}
	// Regular user should not be marked
	if strings.Contains(result.Content.Markdown, "u/other_user [OP]") {
		t.Error("regular user should not be marked with [OP]")
	}
}

func TestHostOf(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.reddit.com/r/test", "www.reddit.com"},
		{"https://old.reddit.com/r/test", "old.reddit.com"},
		{"https://example.com/path?q=1#frag", "example.com"},
		{"http://localhost:8080", "localhost:8080"},
	}
	for _, tt := range tests {
		if got := hostOf(tt.url); got != tt.want {
			t.Errorf("hostOf(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

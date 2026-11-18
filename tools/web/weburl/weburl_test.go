package weburl

import "testing"

func TestKey(t *testing.T) {
	same := []string{
		"https://www.example.com/a/b/",
		"http://example.com/a/b",
		"https://EXAMPLE.com/a/b?utm_source=x&fbclid=1#section",
	}
	for _, u := range same {
		if got := Key(u); got != "example.com/a/b" {
			t.Errorf("Key(%q) = %q", u, got)
		}
	}
	if Key("https://example.com/a?page=2") == Key("https://example.com/a?page=3") {
		t.Error("meaningful query parameters must be kept")
	}
	for _, bad := range []string{"", "mailto:a@b.c", "/relative", "javascript:void(0)"} {
		if Key(bad) != "" {
			t.Errorf("Key(%q) should be empty", bad)
		}
	}
}

func TestHost(t *testing.T) {
	if got := Host("https://www.Fedlex.admin.ch/eli"); got != "fedlex.admin.ch" {
		t.Errorf("Host = %q", got)
	}
}

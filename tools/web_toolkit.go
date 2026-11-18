package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"pedro/documentparser"
	"pedro/tools/web/browser"
	"pedro/tools/web/crawl"
	"pedro/tools/web/extract"
	"pedro/tools/web/fetch"
	"pedro/tools/web/pageindex"
	"pedro/tools/web/reader"
	"pedro/tools/web/search"
)

// browserSession is the interactive browser used by the browser tool.
type browserSession interface {
	Navigate(ctx context.Context, rawURL string) (*browser.State, error)
	Snapshot(ctx context.Context) (*browser.State, error)
	Click(ctx context.Context, ref string) (*browser.State, error)
	Type(ctx context.Context, ref, text string, submit bool) (*browser.State, error)
	Select(ctx context.Context, ref, option string) (*browser.State, error)
	Press(ctx context.Context, key string) (*browser.State, error)
	Scroll(ctx context.Context, direction string) (*browser.State, error)
	Back(ctx context.Context) (*browser.State, error)
	HTML(ctx context.Context) (pageURL, html string, err error)
	Close() error
}

// webToolkit wires the shared web components (Chrome-impersonating HTTP
// client, stealth browser, metasearch, reader, crawler and page index) that
// back the web tools. Everything runs locally.
type webToolkit struct {
	search    search.Searcher
	scholarly search.Searcher
	reader    reader.Reader
	extractor extract.Extractor
	index     pageindex.Index
	crawler   crawl.Crawler
	session   browserSession
	browser   *browser.Manager
}

func newWebToolkit() *webToolkit {
	profile := fetch.ChromeProfile()
	httpClient := fetch.NewBrowserClient(fetch.Options{Profile: profile})
	browserManager := browser.NewManager(browser.Options{})
	extractor := extract.NewDefaultChain()

	var index pageindex.Index = pageindex.Discard{}
	if idx, err := pageindex.NewSQLiteIndex(300); err == nil {
		index = idx
	} else {
		fmt.Println("[Warn] page index unavailable:", err)
	}

	return &webToolkit{
		search: search.NewMetaSearcher(25*time.Second, 10*time.Minute,
			search.Weighted{Provider: search.NewDuckDuckGo(httpClient, browserManager), Weight: 1},
			search.Weighted{Provider: search.NewBrave(httpClient, browserManager), Weight: 1},
			search.Weighted{Provider: search.NewStartpage(httpClient, browserManager), Weight: 1},
			search.Weighted{Provider: search.NewMojeek(httpClient, browserManager), Weight: 0.7},
			search.Weighted{Provider: search.NewWikipedia("en", httpClient), Weight: 0.6},
		),
		scholarly: search.NewMetaSearcher(20*time.Second, 5*time.Minute,
			search.Weighted{Provider: search.NewOpenAlex(httpClient), Weight: 1},
		),
		reader:    reader.New(httpClient, HTTPClient, browserManager, extractor, documentparser.Native{}),
		extractor: extractor,
		index:     index,
		crawler:   crawl.NewColly(fetch.NewBrowserTransport(profile), profile.UserAgent, extractor),
		session:   browser.NewSession(browserManager),
		browser:   browserManager,
	}
}

// Close releases the browser and the page index.
func (k *webToolkit) Close() error {
	return errors.Join(k.session.Close(), k.browser.Close(), k.index.Close())
}

// indexArticle stores a read page so web_find can search it later.
func indexArticle(index pageindex.Index, pageURL string, a *extract.Article) {
	if a == nil || a.Markdown == "" {
		return
	}
	if err := index.Add(pageindex.Page{URL: pageURL, Title: a.Title, Markdown: a.Markdown}); err != nil {
		fmt.Println("[Warn] could not index page:", err)
	}
}

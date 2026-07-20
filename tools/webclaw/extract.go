package webclaw

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Extract extracts structured content from raw HTML.
func Extract(htmlStr string, rawURL *string, options *ExtractionOptions) (*ExtractionResult, error) {
	if htmlStr == "" {
		return nil, ErrNoContent
	}
	if options == nil {
		options = DefaultOptions()
	}

	// Reddit fast path: parse old.reddit.com HTML directly
	if rawURL != nil && isRedditURL(*rawURL) {
		if result := tryExtractReddit(htmlStr, *rawURL); result != nil {
			return result, nil
		}
		// Reddit URL but unrecognisable structure — fall through to generic
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return nil, ErrNoContent
	}

	var baseURL *url.URL
	if rawURL != nil {
		if u, err := url.Parse(*rawURL); err == nil {
			baseURL = u
		}
	}

	exclude := buildExcludeSet(doc, options.ExcludeSelectors)

	// Path 1: include_selectors
	if len(options.IncludeSelectors) > 0 {
		return extractWithInclude(doc, baseURL, options), nil
	}

	// Path 2: only_main_content
	if options.OnlyMainContent {
		if main := doc.Find("article, main, [role='main']").First(); main.Length() > 0 {
			return convertAndRecover(main, doc, baseURL, options, exclude)
		}
	}

	// Path 3: default scoring
	best := findBestNode(doc)
	if best == nil {
		best = doc.Find("body").First()
		if best.Length() == 0 {
			best = doc.Selection
		}
	}

	return convertAndRecover(best, doc, baseURL, options, exclude)
}

func convertAndRecover(
	el *goquery.Selection,
	doc *goquery.Document,
	baseURL *url.URL,
	options *ExtractionOptions,
	exclude map[*goquery.Selection]bool,
) (*ExtractionResult, error) {
	markdown, plainText, assets := convert(el, baseURL, exclude)

	// Recover H1
	if h1 := doc.Find("h1").First(); h1.Length() > 0 {
		h1Text := strings.TrimSpace(h1.Text())
		h1Text = strings.TrimRight(h1Text, "!.?")
		h1Text = strings.TrimSpace(h1Text)
		if h1Text != "" && !strings.Contains(markdown, h1Text) {
			markdown = "# " + h1Text + "\n\n" + markdown
			recoverHeroParagraph(h1, &markdown)
		}
	}

	// Recovery passes
	recoverAnnouncements(doc, baseURL, &markdown, &assets.Links)
	recoverSectionHeadings(doc, &markdown)
	recoverFooterCTA(doc, baseURL, &markdown, &assets.Links)

	// Metadata
	meta := extractMetadata(doc)
	meta.WordCount = wordCount(markdown)

	var rawHTML *string
	if options.IncludeRawHTML {
		if html, err := el.Html(); err == nil {
			rawHTML = strPtr(html)
		}
	}

	return &ExtractionResult{
		Metadata: meta,
		Content: Content{
			Markdown:   markdown,
			PlainText:  plainText,
			Links:      assets.Links,
			Images:     assets.Images,
			CodeBlocks: assets.CodeBlocks,
			RawHTML:    rawHTML,
		},
	}, nil
}

func buildExcludeSet(doc *goquery.Document, selectors []string) map[*goquery.Selection]bool {
	exclude := make(map[*goquery.Selection]bool)
	for _, selStr := range selectors {
		doc.Find(selStr).Each(func(_ int, s *goquery.Selection) {
			exclude[s] = true
			s.Find("*").Each(func(_ int, desc *goquery.Selection) {
				exclude[desc] = true
			})
		})
	}
	return exclude
}

func extractWithInclude(doc *goquery.Document, baseURL *url.URL, options *ExtractionOptions) *ExtractionResult {
	exclude := buildExcludeSet(doc, options.ExcludeSelectors)
	var allMD, allPlain strings.Builder
	var allLinks []Link
	var allImages []Image
	var allCodeBlocks []CodeBlock
	var allRawHTML *string

	for _, selStr := range options.IncludeSelectors {
		doc.Find(selStr).Each(func(_ int, el *goquery.Selection) {
			if exclude[el] {
				return
			}
			md, plain, assets := convert(el, baseURL, exclude)
			if md != "" {
				if allMD.Len() > 0 {
					allMD.WriteString("\n\n")
				}
				allMD.WriteString(md)
			}
			if plain != "" {
				if allPlain.Len() > 0 {
					allPlain.WriteByte('\n')
				}
				allPlain.WriteString(plain)
			}
			allLinks = append(allLinks, assets.Links...)
			allImages = append(allImages, assets.Images...)
			allCodeBlocks = append(allCodeBlocks, assets.CodeBlocks...)
			if options.IncludeRawHTML {
				if html, err := el.Html(); err == nil {
					if allRawHTML == nil {
						allRawHTML = strPtr(html)
					} else {
						s := *allRawHTML + html
						allRawHTML = &s
					}
				}
			}
		})
	}

	meta := extractMetadata(doc)
	meta.WordCount = wordCount(allMD.String())

	return &ExtractionResult{
		Metadata: meta,
		Content: Content{
			Markdown:   allMD.String(),
			PlainText:  allPlain.String(),
			Links:      allLinks,
			Images:     allImages,
			CodeBlocks: allCodeBlocks,
			RawHTML:    allRawHTML,
		},
	}
}

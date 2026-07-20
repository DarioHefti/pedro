package webclaw

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// recoverHeroParagraph finds a substantial <p> near the H1 and inserts it.
func recoverHeroParagraph(h1 *goquery.Selection, markdown *string) {
	node := h1.Parent()
	for i := 0; i < 4; i++ {
		if node.Length() == 0 {
			break
		}
		node.Find("p").Each(func(_ int, p *goquery.Selection) {
			text := strings.TrimSpace(p.Text())
			if len(text) < 40 || len(text) > 300 {
				return
			}
			if strings.Contains(*markdown, text) {
				return
			}
			insert := fmt.Sprintf("\n%s\n", text)
			if idx := strings.Index(*markdown, "\n"); idx >= 0 {
				*markdown = (*markdown)[:idx+1] + insert + (*markdown)[idx+1:]
			} else {
				*markdown += insert
			}
		})
		node = node.Parent()
	}
}

// recoverAnnouncements recovers announcement banners with role="region".
func recoverAnnouncements(doc *goquery.Document, baseURL *url.URL, markdown *string, links *[]Link) {
	doc.Find("[role='region'][aria-label]").Each(func(_ int, el *goquery.Selection) {
		label := el.AttrOr("aria-label", "")
		if !strings.Contains(strings.ToLower(label), "announcement") {
			return
		}
		text := strings.TrimSpace(el.Text())
		text = strings.Join(strings.Fields(text), " ")
		if text == "" || strings.Contains(*markdown, text) {
			return
		}
		announcement := fmt.Sprintf("> **%s**\n\n", text)
		el.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			linkText := strings.TrimSpace(a.Text())
			href := resolveURL(a.AttrOr("href", ""), baseURL)
			if linkText != "" && href != "" {
				*links = append(*links, Link{Text: linkText, Href: href})
			}
		})
		*markdown = announcement + *markdown
	})
}

// recoverSectionHeadings recovers H2s that were stripped as noise.
func recoverSectionHeadings(doc *goquery.Document, markdown *string) {
	doc.Find("h2").Each(func(_ int, h2 *goquery.Selection) {
		h2Text := strings.TrimSpace(h2.Text())
		if h2Text == "" || strings.Contains(*markdown, h2Text) {
			return
		}
		if isInsideStructuralNoise(h2) {
			return
		}
		anchor := findSiblingAnchorText(h2, *markdown)
		if anchor != "" {
			if pos := findContentPosition(*markdown, anchor); pos >= 0 {
				lineStart := 0
				if rIdx := strings.LastIndex((*markdown)[:pos], "\n"); rIdx >= 0 {
					lineStart = rIdx + 1
				}
				headingMD := fmt.Sprintf("## %s\n\n", h2Text)
				*markdown = (*markdown)[:lineStart] + headingMD + (*markdown)[lineStart:]
			}
		}
	})
}

// recoverFooterCTA recovers documentation/app links from footer.
func recoverFooterCTA(doc *goquery.Document, baseURL *url.URL, markdown *string, links *[]Link) {
	doc.Find("footer").Each(func(_ int, footer *goquery.Selection) {
		footer.Find("h2, h3, h4, h5, h6").Each(func(_ int, h *goquery.Selection) {
			text := strings.TrimSpace(h.Text())
			if text == "" || strings.Contains(*markdown, text) {
				return
			}
			lower := strings.ToLower(text)
			if lower == "footer" || lower == "navigation" {
				return
			}
			*markdown += fmt.Sprintf("\n\n## %s\n\n", text)
		})

		footer.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			href := resolveURL(a.AttrOr("href", ""), baseURL)
			text := strings.TrimSpace(a.Text())
			if text == "" || href == "" {
				return
			}
			hrefLower := strings.ToLower(href)
			isValuable := strings.Contains(hrefLower, "docs.") || strings.Contains(hrefLower, "/docs") ||
				strings.Contains(hrefLower, "app.") || strings.Contains(hrefLower, "/app") ||
				strings.Contains(hrefLower, "api.")
			if isValuable && !strings.Contains(*markdown, text) {
				*markdown += fmt.Sprintf("[%s](%s)\n\n", text, href)
				*links = append(*links, Link{Text: text, Href: href})
			}
		})
	})
}

func isInsideStructuralNoise(el *goquery.Selection) bool {
	parent := el.Parent()
	for parent.Length() > 0 {
		tag := goquery.NodeName(parent)
		if tag == "nav" || tag == "aside" || tag == "footer" || tag == "header" {
			return true
		}
		if role, exists := parent.Attr("role"); exists {
			if role == "navigation" || role == "contentinfo" {
				return true
			}
		}
		parent = parent.Parent()
	}
	return false
}

func findSiblingAnchorText(heading *goquery.Selection, markdown string) string {
	headingText := strings.TrimSpace(heading.Text())
	node := heading.Parent()
	for node.Length() > 0 {
		tag := goquery.NodeName(node)
		if tag == "section" || tag == "article" || tag == "main" || tag == "body" {
			var result string
			node.Find("p, h3, h4").Each(func(_ int, el *goquery.Selection) {
				if result != "" {
					return
				}
				elText := strings.Join(strings.Fields(strings.TrimSpace(el.Text())), " ")
				if elText == "" || strings.Contains(headingText, elText) {
					return
				}
				if len(elText) > 15 && strings.Contains(markdown, elText) {
					result = elText
				}
			})
			return result
		}
		node = node.Parent()
	}
	return ""
}

func findContentPosition(markdown, needle string) int {
	searchFrom := 0
	for {
		idx := strings.Index(markdown[searchFrom:], needle)
		if idx < 0 {
			return -1
		}
		return searchFrom + idx
	}
}

package webclaw

import (
	"math"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var candidateSelectors = []string{
	"article", "main", "[role='main']", "div", "section", "td",
}

// findBestNode scores all candidate nodes and returns the highest-scoring one.
func findBestNode(doc *goquery.Document) *goquery.Selection {
	var best *goquery.Selection
	var bestScore float64

	for _, sel := range candidateSelectors {
		doc.Find(sel).Each(func(_ int, el *goquery.Selection) {
			if isNoise(el) || isNoiseDescendant(el) {
				return
			}
			score := scoreNode(el)
			if score > 0 && (best == nil || score > bestScore) {
				best = el
				bestScore = score
			}
		})
	}

	return best
}

// scoreNode assigns a Readability-style score to a DOM element.
func scoreNode(el *goquery.Selection) float64 {
	text := strings.TrimSpace(el.Text())
	textLen := float64(len(text))

	if textLen < 50 {
		return 0
	}

	score := math.Log(textLen)

	// Bonus for semantic tags
	tag := goquery.NodeName(el)
	switch tag {
	case "article", "main":
		score += 50
	}

	// Bonus for role="main"
	if role, exists := el.Attr("role"); exists && role == "main" {
		score += 50
	}

	// Bonus for content-related classes
	if cls, exists := el.Attr("class"); exists {
		cl := strings.ToLower(cls)
		for _, kw := range []string{"content", "article", "post", "entry"} {
			if strings.Contains(cl, kw) {
				score += 25
				break
			}
		}
	}

	// Bonus for content-related IDs
	if id, exists := el.Attr("id"); exists {
		idLower := strings.ToLower(id)
		for _, kw := range []string{"content", "article", "post", "main"} {
			if strings.Contains(idLower, kw) {
				score += 25
				break
			}
		}
	}

	// Paragraph count bonus
	pCount := 0.0
	el.Find("p").Each(func(_ int, s *goquery.Selection) {
		pCount++
	})
	score += pCount * 3

	// Link density penalty
	linkTextLen := 0.0
	el.Find("a").Each(func(_ int, s *goquery.Selection) {
		linkTextLen += float64(len(strings.TrimSpace(s.Text())))
	})

	isSemantic := tag == "article" || tag == "main"
	if textLen > 0 {
		linkDensity := linkTextLen / textLen
		if isSemantic {
			if linkDensity > 0.7 {
				score *= 0.3
			} else if linkDensity > 0.5 {
				score *= 0.5
			}
		} else {
			if linkDensity > 0.5 {
				score *= 0.1
			} else if linkDensity > 0.3 {
				score *= 0.5
			}
		}
	}

	return score
}

// wordCount counts words in text.
func wordCount(text string) int {
	return len(strings.Fields(text))
}

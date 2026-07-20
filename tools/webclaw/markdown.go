package webclaw

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const maxDOMDepth = 200

// ConvertedAssets holds assets found during conversion.
type ConvertedAssets struct {
	Links      []Link
	Images     []Image
	CodeBlocks []CodeBlock
}

// convert transforms an element subtree to markdown + plain text.
func convert(el *goquery.Selection, baseURL *url.URL, exclude map[*goquery.Selection]bool) (string, string, ConvertedAssets) {
	assets := ConvertedAssets{}
	md := nodeToMD(el, baseURL, &assets, 0, exclude, 0)
	plain := stripMarkdown(md)
	md = collapseWhitespace(md)
	plain = collapseWhitespace(plain)
	return md, plain, assets
}

func nodeToMD(el *goquery.Selection, baseURL *url.URL, assets *ConvertedAssets, listDepth int, exclude map[*goquery.Selection]bool, depth int) string {
	if exclude[el] {
		return ""
	}
	if depth > maxDOMDepth {
		return collectText(el)
	}
	if isNoise(el) || isNoiseDescendant(el) {
		collectAssetsFromNoise(el, baseURL, assets)
		return ""
	}

	tag := goquery.NodeName(el)
	switch tag {
	case "h1":
		return fmt.Sprintf("\n\n# %s\n\n", inlineText(el, baseURL, assets, exclude, depth))
	case "h2":
		return fmt.Sprintf("\n\n## %s\n\n", inlineText(el, baseURL, assets, exclude, depth))
	case "h3":
		return fmt.Sprintf("\n\n### %s\n\n", inlineText(el, baseURL, assets, exclude, depth))
	case "h4":
		return fmt.Sprintf("\n\n#### %s\n\n", inlineText(el, baseURL, assets, exclude, depth))
	case "h5":
		return fmt.Sprintf("\n\n##### %s\n\n", inlineText(el, baseURL, assets, exclude, depth))
	case "h6":
		return fmt.Sprintf("\n\n###### %s\n\n", inlineText(el, baseURL, assets, exclude, depth))
	case "p":
		return fmt.Sprintf("\n\n%s\n\n", inlineText(el, baseURL, assets, exclude, depth))
	case "a":
		text := inlineText(el, baseURL, assets, exclude, depth)
		href := resolveURL(el.AttrOr("href", ""), baseURL)
		if text != "" && href != "" {
			assets.Links = append(assets.Links, Link{Text: text, Href: href})
			return fmt.Sprintf("[%s](%s)", text, href)
		}
		return text
	case "img":
		return convertImg(el, baseURL, assets)
	case "strong", "b":
		if cellHasBlockContent(el) {
			return childrenToMD(el, baseURL, assets, listDepth, exclude, depth)
		}
		return fmt.Sprintf("**%s**", inlineText(el, baseURL, assets, exclude, depth))
	case "em", "i":
		if cellHasBlockContent(el) {
			return childrenToMD(el, baseURL, assets, listDepth, exclude, depth)
		}
		return fmt.Sprintf("*%s*", inlineText(el, baseURL, assets, exclude, depth))
	case "code":
		if isInsidePre(el) {
			return collectText(el)
		}
		text := collectText(el)
		if text == "" {
			return ""
		}
		return fmt.Sprintf("`%s`", text)
	case "pre":
		return convertPre(el, assets, depth)
	case "blockquote":
		inner := childrenToMD(el, baseURL, assets, listDepth, exclude, depth)
		lines := strings.Split(strings.TrimSpace(inner), "\n")
		quoted := make([]string, len(lines))
		for i, line := range lines {
			quoted[i] = "> " + line
		}
		return fmt.Sprintf("\n\n%s\n\n", strings.Join(quoted, "\n"))
	case "ul":
		return fmt.Sprintf("\n\n%s\n\n", listItems(el, baseURL, assets, listDepth, false, exclude, depth))
	case "ol":
		return fmt.Sprintf("\n\n%s\n\n", listItems(el, baseURL, assets, listDepth, true, exclude, depth))
	case "li":
		return fmt.Sprintf("- %s\n", inlineText(el, baseURL, assets, exclude, depth))
	case "hr":
		return "\n\n---\n\n"
	case "br":
		return "\n"
	case "table":
		return fmt.Sprintf("\n\n%s\n\n", tableToMD(el, baseURL, assets, exclude, depth))
	default:
		return childrenToMD(el, baseURL, assets, listDepth, exclude, depth)
	}
}

// forEachContentNode iterates over all child nodes (elements + text) of el.
func forEachContentNode(el *goquery.Selection, fn func(child *goquery.Selection, node *html.Node)) {
	for _, node := range el.Nodes {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			childSel := goquery.NewDocumentFromNode(c).Selection
			fn(childSel, c)
		}
	}
}

func childrenToMD(el *goquery.Selection, baseURL *url.URL, assets *ConvertedAssets, listDepth int, exclude map[*goquery.Selection]bool, depth int) string {
	var out strings.Builder
	forEachContentNode(el, func(child *goquery.Selection, node *html.Node) {
		if node.Type == html.ElementNode {
			chunk := nodeToMD(child, baseURL, assets, listDepth, exclude, depth+1)
			if chunk != "" && out.Len() > 0 && needsSeparator(out.String(), chunk) {
				out.WriteByte(' ')
			}
			out.WriteString(chunk)
		} else if node.Type == html.TextNode {
			text := node.Data
			if text != "" && out.Len() > 0 && needsSeparator(out.String(), text) {
				out.WriteByte(' ')
			}
			out.WriteString(text)
		}
	})
	return out.String()
}

func inlineText(el *goquery.Selection, baseURL *url.URL, assets *ConvertedAssets, exclude map[*goquery.Selection]bool, depth int) string {
	var out strings.Builder
	forEachContentNode(el, func(child *goquery.Selection, node *html.Node) {
		if node.Type == html.ElementNode {
			chunk := nodeToMD(child, baseURL, assets, 0, exclude, depth+1)
			if chunk != "" && out.Len() > 0 && needsSeparator(out.String(), chunk) {
				out.WriteByte(' ')
			}
			out.WriteString(chunk)
		} else if node.Type == html.TextNode {
			text := node.Data
			if text != "" && out.Len() > 0 && needsSeparator(out.String(), text) {
				out.WriteByte(' ')
			}
			out.WriteString(text)
		}
	})
	return collapseInlineWhitespace(out.String())
}

func needsSeparator(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	l := left[len(left)-1]
	r := right[0]
	if l == ' ' || l == '\n' || r == ' ' || r == '\n' {
		return false
	}
	if isClosingPunctuation(r) || isOpeningPunctuation(l) {
		return false
	}
	return true
}

func isClosingPunctuation(c byte) bool {
	switch c {
	case '.', ',', ';', ':', '!', '?', ')', ']', '}', '%', '\'', '"':
		return true
	}
	return false
}

func isOpeningPunctuation(c byte) bool {
	switch c {
	case '(', '[', '{', '"':
		return true
	}
	return false
}

func collectText(el *goquery.Selection) string {
	return strings.TrimSpace(el.Text())
}

func collectPreformattedText(el *goquery.Selection, depth int) string {
	if depth > maxDOMDepth {
		return el.Text()
	}
	var out strings.Builder
	forEachContentNode(el, func(child *goquery.Selection, node *html.Node) {
		if node.Type == html.TextNode {
			out.WriteString(node.Data)
		} else if node.Type == html.ElementNode {
			tag := goquery.NodeName(child)
			if tag == "br" {
				out.WriteByte('\n')
			} else if tag == "div" || tag == "p" {
				if out.Len() > 0 {
					s := out.String()
					if s[len(s)-1] != '\n' {
						out.WriteByte('\n')
					}
				}
				out.WriteString(collectPreformattedText(child, depth+1))
				s := out.String()
				if len(s) > 0 && s[len(s)-1] != '\n' {
					out.WriteByte('\n')
				}
			} else {
				out.WriteString(collectPreformattedText(child, depth+1))
			}
		}
	})
	return out.String()
}

func isInsidePre(el *goquery.Selection) bool {
	parent := el.Parent()
	for parent.Length() > 0 {
		if goquery.NodeName(parent) == "pre" {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

func listItems(listEl *goquery.Selection, baseURL *url.URL, assets *ConvertedAssets, depth int, ordered bool, exclude map[*goquery.Selection]bool, domDepth int) string {
	indent := strings.Repeat("  ", depth)
	var out strings.Builder
	index := 1

	forEachContentNode(listEl, func(child *goquery.Selection, node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		if exclude[child] || goquery.NodeName(child) != "li" {
			return
		}

		bullet := "-"
		if ordered {
			bullet = fmt.Sprintf("%d.", index)
			index++
		}

		var inlineParts strings.Builder
		var nestedLists strings.Builder

		forEachContentNode(child, func(liChild *goquery.Selection, liNode *html.Node) {
			if liNode.Type == html.ElementNode {
				if exclude[liChild] {
					return
				}
				childTag := goquery.NodeName(liChild)
				if childTag == "ul" || childTag == "ol" {
					nestedLists.WriteString(listItems(liChild, baseURL, assets, depth+1, childTag == "ol", exclude, domDepth+1))
				} else {
					inlineParts.WriteString(nodeToMD(liChild, baseURL, assets, depth, exclude, domDepth+1))
				}
			} else if liNode.Type == html.TextNode {
				inlineParts.WriteString(liNode.Data)
			}
		})

		text := collapseInlineWhitespace(inlineParts.String())
		out.WriteString(fmt.Sprintf("%s%s %s\n", indent, bullet, text))
		if nestedLists.Len() > 0 {
			out.WriteString(nestedLists.String())
		}
	})
	return strings.TrimRight(out.String(), "\n")
}

func cellHasBlockContent(cell *goquery.Selection) bool {
	blockTags := map[string]bool{
		"p": true, "div": true, "ul": true, "ol": true, "blockquote": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"hr": true, "pre": true, "table": true, "section": true, "article": true,
		"header": true, "footer": true, "nav": true, "aside": true,
	}
	var found bool
	cell.Find("*").Each(func(_ int, s *goquery.Selection) {
		if blockTags[goquery.NodeName(s)] {
			found = true
		}
	})
	return found
}

func tableToMD(tableEl *goquery.Selection, baseURL *url.URL, assets *ConvertedAssets, exclude map[*goquery.Selection]bool, depth int) string {
	type cellEntry struct{ el *goquery.Selection }
	var rawRows [][]cellEntry
	hasHeader := false
	isLayout := false

	tableEl.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		if exclude[tr] {
			return
		}
		var row []cellEntry
		tr.Children().Each(func(_ int, c *goquery.Selection) {
			if exclude[c] {
				return
			}
			tag := goquery.NodeName(c)
			if tag == "th" || tag == "td" {
				if tag == "th" {
					hasHeader = true
				}
				if !isLayout && cellHasBlockContent(c) {
					isLayout = true
				}
				row = append(row, cellEntry{el: c})
			}
		})
		if len(row) > 0 {
			rawRows = append(rawRows, row)
		}
	})

	if len(rawRows) == 0 {
		return ""
	}

	if isLayout {
		var out strings.Builder
		for _, row := range rawRows {
			for _, c := range row {
				content := strings.TrimSpace(childrenToMD(c.el, baseURL, assets, 0, exclude, depth))
				if content != "" {
					if out.Len() > 0 {
						out.WriteString("\n\n")
					}
					out.WriteString(content)
				}
			}
		}
		return out.String()
	}

	var rows [][]string
	for _, rawRow := range rawRows {
		var row []string
		for _, c := range rawRow {
			row = append(row, inlineText(c.el, baseURL, assets, exclude, depth))
		}
		rows = append(rows, row)
	}

	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return ""
	}
	for i := range rows {
		for len(rows[i]) < cols {
			rows[i] = append(rows[i], "")
		}
	}

	var out strings.Builder
	out.WriteString("| ")
	out.WriteString(strings.Join(rows[0], " | "))
	out.WriteString(" |\n")
	seps := make([]string, cols)
	for i := range seps {
		seps[i] = "---"
	}
	out.WriteString("| ")
	out.WriteString(strings.Join(seps, " | "))
	out.WriteString(" |\n")
	start := 0
	if hasHeader {
		start = 1
	}
	for _, row := range rows[start:] {
		out.WriteString("| ")
		out.WriteString(strings.Join(row, " | "))
		out.WriteString(" |\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

func convertImg(el *goquery.Selection, baseURL *url.URL, assets *ConvertedAssets) string {
	alt, _ := el.Attr("alt")
	rawSrc := el.AttrOr("src", "")
	if rawSrc == "" {
		rawSrc = el.AttrOr("data-src", "")
	}
	if rawSrc == "" {
		rawSrc = el.AttrOr("data-lazy-src", "")
	}
	if rawSrc == "" {
		rawSrc = el.AttrOr("data-original", "")
	}
	if strings.HasPrefix(rawSrc, "data:") || strings.HasPrefix(rawSrc, "blob:") {
		return ""
	}

	src := resolveURL(rawSrc, baseURL)
	if src == "" {
		if srcset, exists := el.Attr("srcset"); exists {
			if best := pickBestSrcset(srcset); best != "" {
				src = resolveURL(best, baseURL)
			}
		}
	}

	if src != "" {
		assets.Images = append(assets.Images, Image{Alt: alt, Src: src})
		return fmt.Sprintf("![%s](%s)", alt, src)
	}
	return ""
}

func convertPre(el *goquery.Selection, assets *ConvertedAssets, depth int) string {
	codeEl := el.Find("code").First()
	var code, lang string
	if codeEl.Length() > 0 {
		lang = extractLanguageFromClass(codeEl.AttrOr("class", ""))
		if lang == "" {
			lang = extractLanguageFromClass(el.AttrOr("class", ""))
		}
		code = collectPreformattedText(codeEl, depth)
	} else {
		lang = extractLanguageFromClass(el.AttrOr("class", ""))
		code = collectPreformattedText(el, depth)
	}
	code = strings.Trim(code, "\n")
	assets.CodeBlocks = append(assets.CodeBlocks, CodeBlock{Language: strPtr(lang), Code: code})
	return fmt.Sprintf("\n\n```%s\n%s\n```\n\n", lang, code)
}

func collectAssetsFromNoise(el *goquery.Selection, baseURL *url.URL, assets *ConvertedAssets) {
	el.Find("img[alt]").Each(func(_ int, img *goquery.Selection) {
		alt, _ := img.Attr("alt")
		src := resolveURL(img.AttrOr("src", ""), baseURL)
		if src != "" && alt != "" {
			assets.Images = append(assets.Images, Image{Alt: alt, Src: src})
		}
	})
	el.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := resolveURL(a.AttrOr("href", ""), baseURL)
		text := strings.TrimSpace(a.Text())
		if href != "" && text != "" && strings.HasPrefix(href, "http") {
			assets.Links = append(assets.Links, Link{Text: text, Href: href})
		}
	})
}

func resolveURL(href string, baseURL *url.URL) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//") {
		return href
	}
	if baseURL != nil {
		if resolved, err := baseURL.Parse(href); err == nil {
			return resolved.String()
		}
	}
	return href
}

func pickBestSrcset(srcset string) string {
	bestURL := ""
	bestSize := 0
	for _, entry := range strings.Split(srcset, ",") {
		parts := strings.Fields(entry)
		if len(parts) == 0 {
			continue
		}
		u := parts[0]
		if strings.HasPrefix(u, "data:") || strings.HasPrefix(u, "blob:") {
			continue
		}
		size := 1
		if len(parts) > 1 {
			d := parts[1]
			for _, c := range d {
				if c >= '0' && c <= '9' {
					size = size*10 + int(c-'0')
				} else {
					break
				}
			}
		}
		if size > bestSize {
			bestSize = size
			bestURL = u
		}
	}
	return bestURL
}

var knownLangs = map[string]bool{
	"javascript": true, "typescript": true, "python": true, "rust": true,
	"go": true, "java": true, "c": true, "cpp": true, "csharp": true,
	"ruby": true, "php": true, "swift": true, "kotlin": true, "scala": true,
	"shell": true, "bash": true, "zsh": true, "sql": true, "html": true,
	"css": true, "scss": true, "json": true, "yaml": true, "yml": true,
	"toml": true, "xml": true, "markdown": true, "md": true, "jsx": true,
	"tsx": true, "vue": true, "svelte": true, "graphql": true, "lua": true,
	"perl": true, "r": true, "haskell": true, "elixir": true, "dart": true,
	"zig": true, "diff": true, "text": true, "plaintext": true, "console": true,
}

func extractLanguageFromClass(class string) string {
	for _, cls := range strings.Fields(class) {
		lower := strings.ToLower(cls)
		for _, prefix := range []string{"language-", "lang-", "highlight-"} {
			if strings.HasPrefix(lower, prefix) {
				lang := strings.TrimPrefix(lower, prefix)
				if lang != "" && len(lang) < 20 {
					return normalizeLang(lang)
				}
			}
		}
		if strings.HasPrefix(lower, "sp-") {
			lang := strings.TrimPrefix(lower, "sp-")
			if knownLangs[lang] {
				return normalizeLang(lang)
			}
		}
		if knownLangs[lower] {
			return normalizeLang(lower)
		}
	}
	return ""
}

func normalizeLang(lang string) string {
	switch strings.ToLower(lang) {
	case "javascript", "js":
		return "js"
	case "typescript", "ts":
		return "ts"
	case "python", "py":
		return "python"
	case "csharp", "cs", "c#":
		return "csharp"
	case "cpp", "c++":
		return "cpp"
	case "shell", "bash", "zsh", "sh":
		return "bash"
	case "yaml", "yml":
		return "yaml"
	case "markdown", "md":
		return "markdown"
	case "plaintext", "text":
		return "text"
	default:
		return lang
	}
}

func collapseWhitespace(s string) string {
	var result strings.Builder
	consecutiveNewlines := 0
	inCodeFence := false

	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
			consecutiveNewlines = 0
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteByte('\n')
			}
			result.WriteString(strings.TrimRight(line, " \t"))
			result.WriteByte('\n')
			continue
		}
		if inCodeFence {
			result.WriteString(strings.TrimRight(line, " \t"))
			result.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			consecutiveNewlines++
			if consecutiveNewlines <= 2 {
				result.WriteByte('\n')
			}
		} else {
			consecutiveNewlines = 0
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteByte('\n')
			}
			result.WriteString(strings.TrimRight(line, " \t"))
			result.WriteByte('\n')
		}
	}
	return strings.TrimSpace(result.String())
}

func collapseInlineWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func stripMarkdown(md string) string {
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	imgRe := regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	boldRe := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe := regexp.MustCompile(`\*([^*]+)\*`)
	codeRe := regexp.MustCompile("`([^`]+)`")
	headingRe := regexp.MustCompile(`(?m)^#{1,6}\s+`)
	tableSepRe := regexp.MustCompile(`^\|\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|$`)

	s := imgRe.ReplaceAllString(md, "$1")
	s = linkRe.ReplaceAllString(s, "$1")
	s = boldRe.ReplaceAllString(s, "$1")
	s = italicRe.ReplaceAllString(s, "$1")
	s = codeRe.ReplaceAllString(s, "$1")
	s = headingRe.ReplaceAllString(s, "")

	var lines []string
	inFence := false
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if tableSepRe.MatchString(trimmed) {
			continue
		}
		if len(trimmed) >= 2 && trimmed[0] == '|' && trimmed[len(trimmed)-1] == '|' {
			inner := trimmed[1 : len(trimmed)-1]
			cells := strings.Split(inner, "|")
			for i, c := range cells {
				cells[i] = strings.TrimSpace(c)
			}
			lines = append(lines, strings.Join(cells, "\t"))
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

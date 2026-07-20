package webclaw

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// RedditPost represents a parsed Reddit post.
type RedditPost struct {
	ID           *string `json:"id,omitempty"`
	Title        string  `json:"title"`
	Author       string  `json:"author"`
	Subreddit    *string `json:"subreddit,omitempty"`
	Score        int64   `json:"score"`
	Body         *string `json:"body,omitempty"`
	NumComments  int     `json:"num_comments"`
	Permalink    string  `json:"permalink"`
	URL          *string `json:"url,omitempty"`
	IsSelf       bool    `json:"is_self"`
	Flair        *string `json:"flair,omitempty"`
	CreatedUTC   *string `json:"created_utc,omitempty"`
}

// RedditComment represents a parsed Reddit comment.
type RedditComment struct {
	ID         *string          `json:"id,omitempty"`
	Author     string           `json:"author"`
	Body       string           `json:"body"`
	Score      *int64           `json:"score,omitempty"`
	Depth      int              `json:"depth"`
	IsOP       bool             `json:"is_op"`
	CreatedUTC *string          `json:"created_utc,omitempty"`
	Replies    []RedditComment  `json:"replies"`
}

// RedditThread represents a full Reddit thread.
type RedditThread struct {
	SourceURL string         `json:"url"`
	Post      *RedditPost    `json:"post,omitempty"`
	Comments  []RedditComment `json:"comments"`
}

// isRedditURL checks if a URL is a Reddit thread.
func isRedditURL(url string) bool {
	host := hostOf(url)
	switch host {
	case "reddit.com", "www.reddit.com", "old.reddit.com", "np.reddit.com", "new.reddit.com":
		return true
	}
	return false
}

func hostOf(url string) string {
	after := url
	if idx := strings.Index(url, "://"); idx >= 0 {
		after = url[idx+3:]
	}
	if idx := strings.IndexAny(after, "/?#"); idx >= 0 {
		return after[:idx]
	}
	return after
}

// tryExtractReddit tries to extract a Reddit thread from HTML.
func tryExtractReddit(htmlStr string, url string) *ExtractionResult {
	if !strings.Contains(url, "/comments/") {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return nil
	}

	thread := tryExtractThread(doc, url)
	if thread == nil {
		return nil
	}

	return redditToExtractionResult(thread)
}

// tryExtractThread extracts a Reddit thread from a parsed document.
func tryExtractThread(doc *goquery.Document, url string) *RedditThread {
	post := parsePost(doc)
	op := ""
	if post != nil {
		op = post.Author
	}
	comments := parseComments(doc, op)

	if post == nil && len(comments) == 0 {
		return nil
	}

	return &RedditThread{
		SourceURL: url,
		Post:      post,
		Comments:  comments,
	}
}

// redditToExtractionResult converts a Reddit thread to an ExtractionResult.
func redditToExtractionResult(thread *RedditThread) *ExtractionResult {
	md := redditToMarkdown(thread)
	plain := redditPlainText(md)
	wc := wordCount(md)

	var title, author *string
	if thread.Post != nil {
		title = &thread.Post.Title
		author = &thread.Post.Author
	}

	lang := "en"
	return &ExtractionResult{
		Metadata: Metadata{
			Title:    title,
			Author:   author,
			Language: &lang,
			WordCount: wc,
		},
		Content: Content{
			Markdown:  md,
			PlainText: plain,
		},
		DomainData: &DomainData{DomainType: "social"},
	}
}

// ─── Post parsing ─────────────────────────────────────────────────────────────

func parsePost(doc *goquery.Document) *RedditPost {
	thing := doc.Find("#siteTable .thing.link").First()
	if thing.Length() == 0 {
		return nil
	}

	id := thing.AttrOr("data-fullname", "")
	if id != "" {
		id = strings.TrimPrefix(id, "t3_")
	}

	author := thing.AttrOr("data-author", "[deleted]")
	subreddit := thing.AttrOr("data-subreddit", "")
	score := parseAttrInt64(thing, "data-score")
	numComments := parseAttrInt(thing, "data-comments-count")
	permalinkPath := thing.AttrOr("data-permalink", "")
	permalink := "https://old.reddit.com" + permalinkPath

	isSelf := thing.HasClass("self") ||
		strings.HasPrefix(thing.AttrOr("data-domain", ""), "self.")

	linkURL := thing.AttrOr("data-url", "")
	var url *string
	if !isSelf && linkURL != "" {
		url = &linkURL
	}

	// Title — try nested .title a.title first, then just a.title
	titleEl := thing.Find(".title a.title").First()
	if titleEl.Length() == 0 {
		titleEl = thing.Find("a.title").First()
	}
	title := strings.TrimSpace(titleEl.Text())
	if title == "" {
		return nil
	}

	// Flair
	fairText := strings.TrimSpace(thing.Find(".linkflairlabel").First().Text())
	var flair *string
	if fairText != "" {
		flair = &fairText
	}

	// Self-text body
	var body *string
	entry := thing.Children().Filter(".entry").First()
	if entry.Length() > 0 {
		expando := entry.Children().Filter(".expando").First()
		if expando.Length() > 0 {
			usertext := expando.Children().Filter(".usertext-body").First()
			if usertext.Length() > 0 {
				md := usertext.Children().Filter(".md").First()
				if md.Length() > 0 {
					text := mdToMarkdown(md)
					if text != "" {
						body = &text
					}
				}
			}
		}
	}

	// Datetime
	var createdUTC *string
	timeEl := thing.Find("time[datetime]").First()
	if timeEl.Length() > 0 {
		dt := timeEl.AttrOr("datetime", "")
		if dt != "" {
			createdUTC = &dt
		}
	}

	return &RedditPost{
		ID:          strPtr(id),
		Title:       title,
		Author:      author,
		Subreddit:   strPtr(subreddit),
		Score:       score,
		Body:        body,
		NumComments: numComments,
		Permalink:   permalink,
		URL:         url,
		IsSelf:      isSelf,
		Flair:       flair,
		CreatedUTC:  createdUTC,
	}
}

// ─── Comment parsing ──────────────────────────────────────────────────────────

func parseComments(doc *goquery.Document, op string) []RedditComment {
	// Root listing is .commentarea .sitetable.nestedlisting
	listing := doc.Find(".commentarea .sitetable.nestedlisting").First()
	if listing.Length() == 0 {
		listing = doc.Find(".sitetable.nestedlisting").First()
	}
	if listing.Length() == 0 {
		return nil
	}
	return walkCommentLevel(listing, op, 0)
}

func walkCommentLevel(listing *goquery.Selection, op string, depth int) []RedditComment {
	var comments []RedditComment
	listing.Children().Each(func(_ int, c *goquery.Selection) {
		if !c.HasClass("comment") || !c.HasClass("thing") {
			return
		}
		if comment := parseOneComment(c, op, depth); comment != nil {
			comments = append(comments, *comment)
		}
	})
	return comments
}

func parseOneComment(c *goquery.Selection, op string, depth int) *RedditComment {
	// Skip "load more comments" stubs
	if c.AttrOr("data-type", "") == "morechildren" || c.HasClass("morechildren") {
		return nil
	}

	isDeleted := c.HasClass("deleted")
	id := c.AttrOr("data-fullname", "")
	if id != "" {
		id = strings.TrimPrefix(id, "t1_")
	}

	author := c.AttrOr("data-author", "")
	if author == "" {
		author = "[deleted]"
	}

	// Body: .entry > .usertext-body > .md
	entry := c.Children().Filter(".entry").First()
	var body string
	if entry.Length() > 0 {
		usertext := entry.Children().Filter(".usertext-body").First()
		if usertext.Length() > 0 {
			md := usertext.Children().Filter(".md").First()
			if md.Length() > 0 {
				body = mdToMarkdown(md)
			}
		}
	}

	if body == "" {
		if isDeleted {
			body = "[removed]"
		} else {
			// Try alternate path: .entry > form > .usertext-body > .md
			if entry.Length() > 0 {
				form := entry.Find("form").First()
				if form.Length() > 0 {
					usertext := form.Children().Filter(".usertext-body").First()
					if usertext.Length() > 0 {
						md := usertext.Children().Filter(".md").First()
						if md.Length() > 0 {
							body = mdToMarkdown(md)
						}
					}
				}
			}
		}
	}

	// Score from .score.unvoted span
	var score *int64
	if entry.Length() > 0 {
		scoreSpan := entry.Find("span.score.unvoted").First()
		if scoreSpan.Length() > 0 {
			if title := scoreSpan.AttrOr("title", ""); title != "" {
				if v, err := strconv.ParseInt(title, 10, 64); err == nil {
					score = &v
				}
			}
		}
	}

	// Datetime
	var createdUTC *string
	if entry.Length() > 0 {
		timeEl := entry.Find("time[datetime]").First()
		if timeEl.Length() > 0 {
			dt := timeEl.AttrOr("datetime", "")
			if dt != "" {
				createdUTC = &dt
			}
		}
	}

	isOP := !isDeleted && author != "[deleted]" && author == op

	// Replies: .child > .sitetable > .comment
	var replies []RedditComment
	child := c.Children().Filter(".child").First()
	if child.Length() > 0 {
		sitetable := child.Children().Filter(".sitetable").First()
		if sitetable.Length() > 0 {
			replies = walkCommentLevel(sitetable, op, depth+1)
		}
	}

	return &RedditComment{
		ID:         strPtr(id),
		Author:     author,
		Body:       body,
		Score:      score,
		Depth:      depth,
		IsOP:       isOP,
		CreatedUTC: createdUTC,
		Replies:    replies,
	}
}

// ─── Markdown rendering ───────────────────────────────────────────────────────

func redditToMarkdown(thread *RedditThread) string {
	var out strings.Builder

	if thread.Post != nil {
		p := thread.Post
		out.WriteString(fmt.Sprintf("# %s\n\n", p.Title))

		pts := ptLabel(p.Score)
		cmt := ""
		switch p.NumComments {
		case 1:
			cmt = " · 1 comment"
		default:
			if p.NumComments > 1 {
				cmt = fmt.Sprintf(" · %d comments", p.NumComments)
			}
		}
		sub := "?"
		if p.Subreddit != nil {
			sub = *p.Subreddit
		}
		out.WriteString(fmt.Sprintf("**u/%s** · r/%s · %s%s\n\n", p.Author, sub, pts, cmt))

		if p.Body != nil && *p.Body != "" {
			out.WriteString(*p.Body)
			out.WriteString("\n\n")
		}
		if p.URL != nil && !p.IsSelf {
			out.WriteString(fmt.Sprintf("[Link](%s)\n\n", *p.URL))
		}
		out.WriteString("---\n\n")
	}

	if len(thread.Comments) > 0 {
		out.WriteString("## Comments\n\n")
		for _, c := range thread.Comments {
			renderRedditComment(&c, &out)
		}
	}

	return collapseBlankLines(strings.TrimSpace(out.String()))
}

func renderRedditComment(c *RedditComment, out *strings.Builder) {
	q := strings.Repeat("> ", c.Depth)
	blank := strings.Repeat(">", c.Depth)

	author := fmt.Sprintf("**u/%s**", c.author())
	if c.IsOP {
		author = fmt.Sprintf("**u/%s [OP]**", c.author())
	}
	out.WriteString(fmt.Sprintf("%s%s · %s\n", q, author, ptCommentLabel(c.Score)))

	for _, line := range strings.Split(c.Body, "\n") {
		if line == "" {
			out.WriteString(blank)
			out.WriteByte('\n')
		} else {
			out.WriteString(q)
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	out.WriteByte('\n')

	for _, reply := range c.Replies {
		renderRedditComment(&reply, out)
	}
}

func (c *RedditComment) author() string {
	if c.Author == "" {
		return "[deleted]"
	}
	return c.Author
}

func ptLabel(n int64) string {
	switch n {
	case 1:
		return "1 pt"
	case -1:
		return "-1 pt"
	default:
		return fmt.Sprintf("%d pts", n)
	}
}

func ptCommentLabel(n *int64) string {
	if n == nil {
		return "score hidden"
	}
	return ptLabel(*n)
}

func collapseBlankLines(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	newlines := 0
	for _, ch := range s {
		if ch == '\n' {
			newlines++
			if newlines <= 2 {
				out.WriteByte('\n')
			}
		} else {
			newlines = 0
			out.WriteRune(ch)
		}
	}
	return out.String()
}

func redditPlainText(md string) string {
	var lines []string
	for _, line := range strings.Split(md, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "> ") {
			l = l[2:]
		} else if strings.HasPrefix(l, ">") {
			l = l[1:]
		}
		l = strings.TrimLeft(l, "# ")
		l = strings.ReplaceAll(l, "**", "")
		l = strings.ReplaceAll(l, "~~", "")
		l = strings.ReplaceAll(l, "*", "")
		l = strings.ReplaceAll(l, "`", "")
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n")
}

// ─── Reddit .md div → markdown ────────────────────────────────────────────────

func mdToMarkdown(el *goquery.Selection) string {
	var out strings.Builder
	renderRedditChildren(el, &out)
	return strings.TrimSpace(out.String())
}

func renderRedditChildren(el *goquery.Selection, out *strings.Builder) {
	for _, node := range el.Nodes {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				out.WriteString(c.Data)
			} else if c.Type == html.ElementNode {
				childSel := goquery.NewDocumentFromNode(c).Selection
				renderRedditNode(childSel, out)
			}
		}
	}
}

func renderRedditNode(el *goquery.Selection, out *strings.Builder) {
	tag := goquery.NodeName(el)
	switch tag {
	case "p", "div":
		var inner strings.Builder
		renderRedditChildren(el, &inner)
		t := strings.TrimSpace(inner.String())
		if t != "" {
			out.WriteString(t)
			out.WriteString("\n\n")
		}
	case "br":
		out.WriteByte('\n')
	case "strong", "b":
		t := strings.TrimSpace(el.Text())
		if t != "" {
			out.WriteString(fmt.Sprintf("**%s**", t))
		}
	case "em", "i":
		t := strings.TrimSpace(el.Text())
		if t != "" {
			out.WriteString(fmt.Sprintf("*%s*", t))
		}
	case "del", "s", "strike":
		t := strings.TrimSpace(el.Text())
		if t != "" {
			out.WriteString(fmt.Sprintf("~~%s~~", t))
		}
	case "code":
		t := strings.TrimSpace(el.Text())
		out.WriteString("`" + t + "`")
	case "pre":
		t := el.Text()
		t = strings.TrimRight(t, "\n")
		out.WriteString("```\n" + t + "\n```\n\n")
	case "a":
		text := strings.TrimSpace(el.Text())
		if text == "" {
			return
		}
		href := el.AttrOr("href", "")
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
			out.WriteString(fmt.Sprintf("[%s](%s)", text, href))
		} else if strings.HasPrefix(href, "/") {
			out.WriteString(fmt.Sprintf("[%s](https://old.reddit.com%s)", text, href))
		} else {
			out.WriteString(text)
		}
	case "blockquote":
		var inner strings.Builder
		renderRedditChildren(el, &inner)
		trimmed := strings.TrimSpace(inner.String())
		for _, line := range strings.Split(trimmed, "\n") {
			out.WriteByte('>')
			if line != "" {
				out.WriteByte(' ')
				out.WriteString(line)
			}
			out.WriteByte('\n')
		}
		out.WriteByte('\n')
	case "ul":
		renderRedditList(el, false, 0, out)
	case "ol":
		renderRedditList(el, true, 0, out)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := 2
		if len(tag) > 1 {
			if d, err := strconv.Atoi(tag[1:]); err == nil {
				level = d
			}
		}
		t := strings.TrimSpace(el.Text())
		if t != "" {
			out.WriteString(strings.Repeat("#", level) + " " + t + "\n\n")
		}
	case "hr":
		out.WriteString("---\n\n")
	case "sup":
		t := strings.TrimSpace(el.Text())
		out.WriteString(t)
	default:
		renderRedditChildren(el, out)
	}
}

func renderRedditList(list *goquery.Selection, ordered bool, indent int, out *strings.Builder) {
	pad := strings.Repeat("  ", indent)
	n := 0
	for _, li := range list.Children().Nodes {
		if li.Type != html.ElementNode {
			continue
		}
		liSel := goquery.NewDocumentFromNode(li).Selection
		if goquery.NodeName(liSel) != "li" {
			continue
		}
		n++

		// Inline content (excluding nested lists)
		var inline strings.Builder
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				inline.WriteString(c.Data)
			} else if c.Type == html.ElementNode {
				childTag := goquery.NodeName(goquery.NewDocumentFromNode(c).Selection)
				if childTag == "ul" || childTag == "ol" {
					continue
				}
				renderRedditNode(goquery.NewDocumentFromNode(c).Selection, &inline)
			}
		}

		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", n)
		}
		out.WriteString(fmt.Sprintf("%s%s%s\n", pad, marker, strings.TrimSpace(inline.String())))

		// Nested lists
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				childSel := goquery.NewDocumentFromNode(c).Selection
				childTag := goquery.NodeName(childSel)
				if childTag == "ul" {
					renderRedditList(childSel, false, indent+1, out)
				} else if childTag == "ol" {
					renderRedditList(childSel, true, indent+1, out)
				}
			}
		}
	}
	if indent == 0 {
		out.WriteByte('\n')
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func parseAttrInt64(el *goquery.Selection, attr string) int64 {
	v := el.AttrOr(attr, "")
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func parseAttrInt(el *goquery.Selection, attr string) int {
	v := el.AttrOr(attr, "")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

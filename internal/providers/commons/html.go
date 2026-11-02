package commons

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// blockTags separate words when an HTML fragment is turned into text.
var blockTags = map[string]bool{
	"br": true, "li": true, "p": true, "div": true, "ul": true, "ol": true,
	"td": true, "th": true, "tr": true, "table": true,
}

// link is an <a> element of an HTML fragment.
type link struct {
	href string // absolute http(s) URL, or "" if missing or unusable
	text string // visible text
	// redlink marks links to wiki pages that do not exist.
	redlink bool
}

// parseHTML returns the visible text of an HTML fragment, with whitespace
// collapsed, and its links. Commons metadata such as Artist and
// ImageDescription is HTML; it is never passed to the UI as markup.
func parseHTML(fragment string) (string, []link) {
	var text strings.Builder
	var links []link
	var current *link // the <a> being read
	var linkText strings.Builder

	z := html.NewTokenizer(strings.NewReader(fragment))
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return collapse(text.String()), links
		case html.TextToken:
			// Text consumes the token's text: a second call returns nothing.
			t := z.Text()
			text.Write(t)
			if current != nil {
				linkText.Write(t)
			}
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			name, hasAttr := z.TagName()
			if blockTags[string(name)] {
				text.WriteByte(' ')
			}
			if string(name) != "a" {
				continue
			}
			if tt == html.EndTagToken {
				if current != nil {
					current.text = collapse(linkText.String())
					links = append(links, *current)
					current = nil
				}
				continue
			}
			l := link{}
			if hasAttr {
				l.href, l.redlink = hrefAttr(z)
			}
			current = &l
			linkText.Reset()
		}
	}
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func hrefAttr(z *html.Tokenizer) (string, bool) {
	for {
		key, val, more := z.TagAttr()
		if string(key) == "href" {
			return absoluteLink(string(val))
		}
		if !more {
			return "", false
		}
	}
}

// absoluteLink makes protocol-relative links absolute and accepts only
// http(s) URLs. It also reports links to missing wiki pages (redlinks).
func absoluteLink(href string) (string, bool) {
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", false
	}
	return u.String(), u.Query().Get("redlink") == "1"
}

// maxAuthorName is the longest Artist text that is used as a name when it
// contains no link to a person.
const maxAuthorName = 100

// parseArtist splits Commons' Artist HTML into the author's name, a link
// to their page and, when the field says more than the name, the full text
// as a credit note. Many photographers use templates with icons, links to
// the file and usage conditions, so the first link is often not the author;
// the author is the first link to a person (user page, creator page,
// Wikidata item or Wikipedia article). Nothing is dropped: if the name is
// not the whole text, the whole text is the credit.
func parseArtist(fragment string) (name, profile, credit string) {
	text, links := parseHTML(fragment)
	for _, l := range links {
		if l.text != "" && isPersonLink(l.href) {
			name = l.text
			if !l.redlink {
				profile = l.href
			}
			break
		}
	}
	if name == "" && utf8.RuneCountInString(text) <= maxAuthorName {
		name = text
	}
	if text != name {
		credit = text
	}
	return name, profile, credit
}

// isPersonLink reports whether href points at a page about a person.
func isPersonLink(href string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	host := u.Hostname()
	page := strings.TrimPrefix(u.Path, "/wiki/")
	if page == u.Path { // not a /wiki/ link; try index.php?title=
		page = u.Query().Get("title")
	}
	switch {
	case host == "commons.wikimedia.org":
		return strings.HasPrefix(page, "User:") || strings.HasPrefix(page, "Creator:")
	case host == "www.wikidata.org" || host == "wikidata.org":
		return strings.HasPrefix(page, "Q")
	case strings.HasSuffix(host, ".wikipedia.org"):
		return page != "" && !strings.Contains(page, ":") // articles, not File:, Help:, …
	}
	return false
}

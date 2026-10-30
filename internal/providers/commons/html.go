package commons

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// blockTags separate words when an HTML fragment is turned into text.
var blockTags = map[string]bool{"br": true, "li": true, "p": true, "div": true, "ul": true, "ol": true, "td": true}

// parseHTML returns the visible text of an HTML fragment, with whitespace
// collapsed, and the first link to an existing http(s) page. Commons
// metadata such as Artist and ImageDescription is HTML; it is never passed
// to the UI as markup. Protocol-relative links are made absolute and links
// to missing wiki pages (redlinks) are skipped.
func parseHTML(fragment string) (text, link string) {
	var b strings.Builder
	z := html.NewTokenizer(strings.NewReader(fragment))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return strings.Join(strings.Fields(b.String()), " "), link
		case html.TextToken:
			b.Write(z.Text())
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			name, hasAttr := z.TagName()
			if blockTags[string(name)] {
				b.WriteByte(' ')
			}
			if string(name) == "a" && hasAttr && link == "" {
				link = hrefAttr(z)
			}
		}
	}
}

func hrefAttr(z *html.Tokenizer) string {
	for {
		key, val, more := z.TagAttr()
		if string(key) == "href" {
			return absoluteLink(string(val))
		}
		if !more {
			return ""
		}
	}
}

func absoluteLink(href string) string {
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	if u.Query().Get("redlink") == "1" {
		return ""
	}
	return u.String()
}

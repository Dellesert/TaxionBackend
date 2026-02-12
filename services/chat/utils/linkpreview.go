package utils

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"tachyon-messenger/services/chat/models"

	"golang.org/x/net/html"
)

var urlRegex = regexp.MustCompile(`https?://[^\s<>"]+`)

// ExtractFirstURL extracts the first HTTP/HTTPS URL from text content.
func ExtractFirstURL(content string) string {
	match := urlRegex.FindString(content)
	// Trim trailing punctuation that's likely not part of the URL
	match = strings.TrimRight(match, ".,;:!?)]}>\"'")
	return match
}

// FetchLinkPreview fetches Open Graph metadata from a URL.
func FetchLinkPreview(rawURL string) (*models.LinkPreview, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TaxionBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml") {
		return nil, fmt.Errorf("not an HTML page: %s", contentType)
	}

	// Read at most 512KB to avoid downloading huge pages
	limitedReader := io.LimitReader(resp.Body, 512*1024)
	doc, err := html.Parse(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	preview := &models.LinkPreview{
		URL: rawURL,
	}

	var titleFromTag string
	var descFromMeta string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				parseMeta(n, preview, &descFromMeta)
			case "title":
				if n.FirstChild != nil {
					titleFromTag = strings.TrimSpace(n.FirstChild.Data)
				}
			case "body":
				// Stop parsing once we hit <body> - all meta should be in <head>
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Fallback: use <title> tag if og:title is empty
	if preview.Title == "" {
		preview.Title = titleFromTag
	}

	// Fallback: use meta description if og:description is empty
	if preview.Description == "" {
		preview.Description = descFromMeta
	}

	// If we couldn't extract anything useful, return nil
	if preview.Title == "" && preview.Description == "" && preview.Image == "" {
		return nil, fmt.Errorf("no metadata found")
	}

	// Truncate description to 300 chars
	if len(preview.Description) > 300 {
		preview.Description = preview.Description[:297] + "..."
	}

	return preview, nil
}

// parseMeta extracts Open Graph and standard meta tags
func parseMeta(n *html.Node, preview *models.LinkPreview, descFromMeta *string) {
	var property, name, content string
	for _, attr := range n.Attr {
		switch attr.Key {
		case "property":
			property = attr.Val
		case "name":
			name = attr.Val
		case "content":
			content = attr.Val
		}
	}

	if content == "" {
		return
	}

	switch property {
	case "og:title":
		preview.Title = content
	case "og:description":
		preview.Description = content
	case "og:image":
		preview.Image = content
	case "og:site_name":
		preview.SiteName = content
	}

	// Standard meta tags as fallback
	switch name {
	case "description":
		*descFromMeta = content
	}
}

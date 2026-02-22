package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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

const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// FetchLinkPreview fetches Open Graph metadata from a URL.
func FetchLinkPreview(rawURL string) (*models.LinkPreview, error) {
	return fetchLinkPreviewWithDepth(rawURL, 0)
}

// fetchLinkPreviewWithDepth fetches link preview, following meta refresh redirects up to 3 levels deep.
func fetchLinkPreviewWithDepth(rawURL string, depth int) (*models.LinkPreview, error) {
	if depth > 3 {
		return nil, fmt.Errorf("too many meta refresh redirects")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Connection", "keep-alive")

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

	// Use the final URL after HTTP redirects
	finalURL := resp.Request.URL.String()

	// Read at most 512KB to avoid downloading huge pages
	limitedReader := io.LimitReader(resp.Body, 512*1024)
	doc, err := html.Parse(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	preview := &models.LinkPreview{
		URL: finalURL,
	}

	var titleFromTag string
	var descFromMeta string
	var metaRefreshURL string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				parseMeta(n, preview, &descFromMeta)
				if refresh := extractMetaRefresh(n); refresh != "" {
					metaRefreshURL = refresh
				}
			case "title":
				if n.FirstChild != nil {
					titleFromTag = strings.TrimSpace(n.FirstChild.Data)
				}
			case "body":
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// If we found a meta refresh redirect and no useful OG data, follow it
	if metaRefreshURL != "" && preview.Title == "" && preview.Image == "" {
		resolved := resolveURL(finalURL, metaRefreshURL)
		if resolved != "" && resolved != finalURL && resolved != rawURL {
			return fetchLinkPreviewWithDepth(resolved, depth+1)
		}
	}

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

// extractMetaRefresh extracts the redirect URL from a <meta http-equiv="refresh"> tag.
func extractMetaRefresh(n *html.Node) string {
	var httpEquiv, content string
	for _, attr := range n.Attr {
		switch strings.ToLower(attr.Key) {
		case "http-equiv":
			httpEquiv = strings.ToLower(attr.Val)
		case "content":
			content = attr.Val
		}
	}
	if httpEquiv != "refresh" || content == "" {
		return ""
	}
	// Format: "0; url=https://example.com" or "0;url=https://example.com"
	parts := strings.SplitN(content, "url=", 2)
	if len(parts) != 2 {
		parts = strings.SplitN(content, "URL=", 2)
	}
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(strings.Trim(parts[1], "'\""))
}

// resolveURL resolves a potentially relative URL against a base URL.
func resolveURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(refURL).String()
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

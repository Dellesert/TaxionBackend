package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
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

const (
	// Social media bot UA - whitelisted by most sites (Ozon, etc.) for OG tag access
	socialBotUserAgent = "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)"
	// Browser UA as fallback
	browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// FetchLinkPreview fetches Open Graph metadata from a URL.
// It tries social bot UA first (whitelisted by most sites), then falls back to browser UA.
func FetchLinkPreview(rawURL string) (*models.LinkPreview, error) {
	// First attempt: social bot UA (works for Ozon, most e-commerce, social media, etc.)
	preview, err := fetchWithUA(rawURL, socialBotUserAgent)
	if err == nil {
		return preview, nil
	}

	// Second attempt: browser UA with cookie jar (works for sites that block social bots)
	preview, err = fetchWithUA(rawURL, browserUserAgent)
	if err == nil {
		return preview, nil
	}

	// Last resort: try to generate preview from URL slug
	if preview := previewFromURLSlug(rawURL); preview != nil {
		return preview, nil
	}

	return nil, err
}

// fetchWithUA attempts to fetch link preview with a specific User-Agent.
// Follows HTTP redirects and meta refresh redirects.
func fetchWithUA(rawURL, userAgent string) (*models.LinkPreview, error) {
	return fetchWithDepth(rawURL, userAgent, 0)
}

func fetchWithDepth(rawURL, userAgent string, depth int) (*models.LinkPreview, error) {
	if depth > 3 {
		return nil, fmt.Errorf("too many meta refresh redirects")
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
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
	req.Header.Set("User-Agent", userAgent)
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
			return fetchWithDepth(resolved, userAgent, depth+1)
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

// previewFromURLSlug generates a basic preview from URL path when fetching fails.
// Extracts readable product/page names from URL slugs (e.g. "product/super-cool-item-123").
func previewFromURLSlug(rawURL string) *models.LinkPreview {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	host := parsed.Hostname()
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return nil
	}

	// Find the longest meaningful path segment (likely the product/page name)
	segments := strings.Split(path, "/")
	var bestSlug string
	for _, seg := range segments {
		// Skip short segments, numeric IDs, and common path segments
		if len(seg) < 5 {
			continue
		}
		if seg == "item" || seg == "product" || seg == "catalog" || seg == "category" {
			continue
		}
		if len(seg) > len(bestSlug) {
			bestSlug = seg
		}
	}

	if bestSlug == "" {
		return nil
	}

	// Clean up slug: remove file extensions, trailing IDs, and convert to readable text
	bestSlug = strings.TrimSuffix(bestSlug, ".html")
	bestSlug = strings.TrimSuffix(bestSlug, ".htm")
	bestSlug = strings.ReplaceAll(bestSlug, "-", " ")
	bestSlug = strings.ReplaceAll(bestSlug, "_", " ")

	// Remove trailing numeric ID (common in e-commerce URLs)
	if idx := strings.LastIndex(bestSlug, " "); idx > 0 {
		tail := bestSlug[idx+1:]
		allDigits := true
		for _, c := range tail {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits && len(tail) > 4 {
			bestSlug = bestSlug[:idx]
		}
	}

	bestSlug = strings.TrimSpace(bestSlug)
	if bestSlug == "" {
		return nil
	}

	// Capitalize first letter
	title := strings.ToUpper(bestSlug[:1]) + bestSlug[1:]

	// Determine site name from host
	siteName := hostToSiteName(host)

	return &models.LinkPreview{
		URL:      rawURL,
		Title:    title,
		SiteName: siteName,
	}
}

// hostToSiteName returns a friendly site name for known hosts.
func hostToSiteName(host string) string {
	host = strings.TrimPrefix(host, "www.")
	switch {
	case strings.Contains(host, "aliexpress"):
		return "AliExpress"
	case strings.Contains(host, "ali.click"):
		return "AliExpress"
	case strings.Contains(host, "ozon"):
		return "Ozon"
	case strings.Contains(host, "wildberries") || strings.Contains(host, "wb.ru"):
		return "Wildberries"
	case strings.Contains(host, "market.yandex") || strings.Contains(host, "ya.cc"):
		return "Яндекс Маркет"
	case strings.Contains(host, "avito"):
		return "Авито"
	case strings.Contains(host, "dns-shop"):
		return "DNS"
	case strings.Contains(host, "citilink"):
		return "Ситилинк"
	case strings.Contains(host, "mvideo"):
		return "М.Видео"
	case strings.Contains(host, "lamoda"):
		return "Lamoda"
	default:
		return ""
	}
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
	lower := strings.ToLower(content)
	idx := strings.Index(lower, "url=")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(strings.Trim(content[idx+4:], "'\""))
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

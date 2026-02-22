package utils

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

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
// Strategy: resolve short URLs first, then try social bot UA, then browser UA, then URL slug fallback.
func FetchLinkPreview(rawURL string) (*models.LinkPreview, error) {
	// Step 1: resolve short/redirect URLs to get the real destination
	resolvedURL := resolveRedirectURL(rawURL)
	log.Printf("[LinkPreview] URL: %s -> resolved: %s", rawURL, resolvedURL)

	// Step 2: try social bot UA on the resolved URL (works for Ozon, most sites)
	preview, err := fetchWithUA(resolvedURL, socialBotUserAgent)
	if err == nil {
		log.Printf("[LinkPreview] Success with social bot UA for %s", resolvedURL)
		return preview, nil
	}
	log.Printf("[LinkPreview] Social bot UA failed for %s: %v", resolvedURL, err)

	// Step 3: if resolved URL differs, also try social bot on the original
	if resolvedURL != rawURL {
		preview, err = fetchWithUA(rawURL, socialBotUserAgent)
		if err == nil {
			log.Printf("[LinkPreview] Success with social bot UA for original %s", rawURL)
			return preview, nil
		}
		log.Printf("[LinkPreview] Social bot UA failed for original %s: %v", rawURL, err)
	}

	// Step 4: browser UA with cookie jar
	preview, err = fetchWithUA(resolvedURL, browserUserAgent)
	if err == nil {
		log.Printf("[LinkPreview] Success with browser UA for %s", resolvedURL)
		return preview, nil
	}
	log.Printf("[LinkPreview] Browser UA failed for %s: %v", resolvedURL, err)

	// Step 5: generate preview from URL slug as last resort
	if preview := previewFromURLSlug(resolvedURL); preview != nil {
		log.Printf("[LinkPreview] Using URL slug fallback for %s: title=%q", resolvedURL, preview.Title)
		return preview, nil
	}

	return nil, err
}

// resolveRedirectURL captures the destination from the FIRST redirect hop only.
// This works for short URL services (ozon.ru/t/..., ali.click/...) because
// the first 301/302 redirect always works, even from blocked IPs.
// Anti-bot protection only kicks in on the destination page, not the redirect.
func resolveRedirectURL(rawURL string) string {
	// Collect all redirect URLs without following them fully
	var redirectURLs []string

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			redirectURLs = append(redirectURLs, req.URL.String())
			// Stop after collecting first meaningful redirect
			// (skip same-host redirects like http->https or www addition)
			if len(via) >= 1 && req.URL.Host != via[0].URL.Host {
				return http.ErrUseLastResponse
			}
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return rawURL
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Also check Location header from the final response (in case we stopped early)
	if resp != nil {
		if loc := resp.Header.Get("Location"); loc != "" {
			resolved := resolveURL(rawURL, loc)
			if resolved != "" {
				redirectURLs = append(redirectURLs, resolved)
			}
		}
	}

	// If we hit an error but already captured redirect URLs, that's fine
	if len(redirectURLs) == 0 {
		return rawURL
	}

	// Use the last (deepest) captured redirect URL
	finalURL := redirectURLs[len(redirectURLs)-1]

	// Clean up tracking params
	if parsed, parseErr := url.Parse(finalURL); parseErr == nil {
		q := parsed.Query()
		q.Del("__rr")
		parsed.RawQuery = q.Encode()
		finalURL = parsed.String()
	}

	if finalURL != "" && finalURL != rawURL {
		return finalURL
	}
	return rawURL
}

// fetchWithUA attempts to fetch link preview with a specific User-Agent.
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

// previewFromURLSlug generates a basic preview from URL path when all fetching fails.
// Only produces a preview if the slug looks like a human-readable product/page name.
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

	// Find the best readable slug from path segments
	segments := strings.Split(path, "/")
	var bestSlug string
	for _, seg := range segments {
		// Skip common non-descriptive segments
		if seg == "item" || seg == "product" || seg == "catalog" || seg == "category" || seg == "t" || seg == "dp" {
			continue
		}

		cleaned := seg
		cleaned = strings.TrimSuffix(cleaned, ".html")
		cleaned = strings.TrimSuffix(cleaned, ".htm")

		// A readable slug must contain hyphens or underscores (word separators)
		// This filters out random codes like "vsFvzhB", "xd7c018", "1005008603585460"
		if !strings.Contains(cleaned, "-") && !strings.Contains(cleaned, "_") {
			continue
		}

		cleaned = strings.ReplaceAll(cleaned, "-", " ")
		cleaned = strings.ReplaceAll(cleaned, "_", " ")

		// Must have at least 2 words and be reasonably long
		words := strings.Fields(cleaned)
		if len(words) < 2 || len(cleaned) < 10 {
			continue
		}

		// Check that it's not mostly numbers
		letterCount := 0
		for _, r := range cleaned {
			if unicode.IsLetter(r) {
				letterCount++
			}
		}
		if letterCount < len(cleaned)/3 {
			continue
		}

		if len(cleaned) > len(bestSlug) {
			bestSlug = cleaned
		}
	}

	if bestSlug == "" {
		return nil
	}

	// Remove trailing numeric ID (common in e-commerce URLs)
	words := strings.Fields(bestSlug)
	if len(words) > 1 {
		lastWord := words[len(words)-1]
		allDigits := true
		for _, c := range lastWord {
			if !unicode.IsDigit(c) {
				allDigits = false
				break
			}
		}
		if allDigits && len(lastWord) > 4 {
			words = words[:len(words)-1]
			bestSlug = strings.Join(words, " ")
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

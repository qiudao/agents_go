package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// ── DuckDuckGo Search (no API key needed) ────────────────────────────────

func webSearch(query string) string {
	u := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AgentsGo/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Error reading response: %v", err)
	}
	html := string(body)

	// Parse results from DuckDuckGo HTML
	type searchResult struct {
		Title   string
		Link    string
		Snippet string
	}

	var results []searchResult

	// Extract result blocks: <a class="result__a" href="...">title</a>
	linkRe := regexp.MustCompile(`<a[^>]+class="result__a"[^>]+href="([^"]*)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`<a[^>]+class="result__snippet"[^>]*>(.*?)</a>`)

	links := linkRe.FindAllStringSubmatch(html, 10)
	snippets := snippetRe.FindAllStringSubmatch(html, 10)

	for i, m := range links {
		if len(m) < 3 {
			continue
		}
		link := m[1]
		// DuckDuckGo wraps URLs in a redirect; extract the actual URL
		if idx := strings.Index(link, "uddg="); idx >= 0 {
			decoded, err := url.QueryUnescape(link[idx+5:])
			if err == nil {
				// Strip any trailing &rut=...
				if ampIdx := strings.Index(decoded, "&"); ampIdx >= 0 {
					decoded = decoded[:ampIdx]
				}
				link = decoded
			}
		}
		title := stripTags(m[2])
		snippet := ""
		if i < len(snippets) && len(snippets[i]) >= 2 {
			snippet = stripTags(snippets[i][1])
		}
		results = append(results, searchResult{Title: title, Link: link, Snippet: snippet})
		if len(results) >= 5 {
			break
		}
	}

	if len(results) == 0 {
		return "No results found."
	}

	// Build raw results text
	var sb strings.Builder
	for i, item := range results {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n\n", i+1, item.Title, item.Link, item.Snippet)
	}
	raw := strings.TrimSpace(sb.String())

	// Summarize with small model
	prompt := fmt.Sprintf("Based on these search results for query \"%s\", provide a concise summary of the key findings. Keep the source URLs for reference.\n\n%s", query, raw)
	summary := summarizeWithSmallModel(raw, prompt, "web_search://"+query)
	return summary
}

// stripTags removes HTML tags from a string.
func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	s = re.ReplaceAllString(s, "")
	s = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&nbsp;", " ").Replace(s)
	return strings.TrimSpace(s)
}

// ── Web Fetch ────────────────────────────────────────────────────────────

func webFetch(fetchURL, prompt string) string {
	// 1. HTTP GET
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AgentsGo/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Error: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 512KB max
	if err != nil {
		return fmt.Sprintf("Error reading body: %v", err)
	}
	body := string(bodyBytes)

	// 2. HTML → plain text
	text := htmlToText(body)
	if len(text) < 50 {
		return "Error: page content too short or empty"
	}

	// 3. Use small model to extract key information
	if prompt == "" {
		prompt = "Extract and summarize the key information from this web page. Return the essential content in a clear, organized format."
	}

	summary := summarizeWithSmallModel(text, prompt, fetchURL)
	return summary
}

// htmlToText does basic HTML → plain text conversion.
func htmlToText(html string) string {
	// Remove script, style, nav, header, footer tags and their content
	for _, tag := range []string{"script", "style", "nav", "header", "footer", "noscript", "svg"} {
		re := regexp.MustCompile(`(?is)<` + tag + `[^>]*>.*?</` + tag + `>`)
		html = re.ReplaceAllString(html, "")
	}

	// Convert common block elements to newlines
	for _, tag := range []string{"p", "div", "br", "li", "h1", "h2", "h3", "h4", "h5", "h6", "tr", "blockquote"} {
		re := regexp.MustCompile(`(?i)</?` + tag + `[^>]*>`)
		html = re.ReplaceAllString(html, "\n")
	}

	// Remove all remaining HTML tags
	re := regexp.MustCompile(`<[^>]+>`)
	html = re.ReplaceAllString(html, "")

	// Decode common HTML entities
	replacer := strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", "\"", "&#39;", "'", "&apos;", "'",
		"&nbsp;", " ", "&mdash;", "—", "&ndash;", "–",
	)
	html = replacer.Replace(html)

	// Collapse whitespace
	html = regexp.MustCompile(`[ \t]+`).ReplaceAllString(html, " ")
	html = regexp.MustCompile(`\n{3,}`).ReplaceAllString(html, "\n\n")

	return strings.TrimSpace(html)
}

// summarizeWithSmallModel calls a small LLM to extract key content.
func summarizeWithSmallModel(text, prompt, sourceURL string) string {
	// Determine summary model from config
	summaryProvider := ""
	summaryModel := ""

	if v := os.Getenv("SUMMARY_PROVIDER"); v != "" {
		summaryProvider = v
	}
	if v := os.Getenv("SUMMARY_MODEL"); v != "" {
		summaryModel = v
	}

	if summaryProvider == "" || summaryModel == "" {
		cfg := loadConfig()
		if summaryProvider == "" {
			summaryProvider = cfg["SUMMARY_PROVIDER"]
		}
		if summaryModel == "" {
			summaryModel = cfg["SUMMARY_MODEL"]
		}
	}

	// Defaults: DeepSeek
	if summaryProvider == "" {
		summaryProvider = "deepseek"
	}
	if summaryModel == "" {
		switch summaryProvider {
		case "deepseek":
			summaryModel = "deepseek-chat"
		case "gemini":
			summaryModel = "gemini-2.5-flash"
		case "anthropic":
			summaryModel = "claude-haiku-4-5-20251001"
		}
	}

	// Cap text sent to summary model (keep ~30K chars to stay within context)
	if len(text) > 30000 {
		text = text[:30000]
	}

	provider, err := newProvider(summaryProvider, summaryModel)
	if err != nil {
		// Fallback: return truncated raw text
		return fmt.Sprintf("[Could not load summary model (%v), returning raw text]\n\n%s", err, truncate(text, 5000))
	}

	messages := []Message{
		{
			Role: "user",
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("%s\n\nSource URL: %s\n\n---\n\n%s", prompt, sourceURL, text),
			}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.Chat(ctx, messages, nil)
	if err != nil {
		return fmt.Sprintf("[Summary model error: %v]\n\n%s", err, truncate(text, 5000))
	}

	var sb strings.Builder
	for _, b := range resp.Content {
		if b.Type == "text" {
			sb.WriteString(b.Text)
		}
	}

	result := sb.String()
	if result == "" {
		return truncate(text, 5000)
	}
	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n...(truncated)"
}

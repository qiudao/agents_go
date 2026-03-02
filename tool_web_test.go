package main

import (
	"io"
	"net/http"
	"testing"
	"time"
)

const ddgLiteSample = `
<table border="0">
  <tr>
    <td valign="top">1.&nbsp;</td>
    <td>
      <a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fpkg.go.dev%2Fcontext&amp;rut=abc123" class='result-link'>context package - context - Go Packages</a>
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td class='result-snippet'>
      Package <b>context</b> defines the Context type, which carries deadlines.
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td>
      <span class='link-text'>pkg.go.dev/context</span>
    </td>
  </tr>
  <tr><td>&nbsp;</td><td>&nbsp;</td></tr>

  <tr>
    <td valign="top">2.&nbsp;</td>
    <td>
      <a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.digitalocean.com%2Fcommunity%2Ftutorials%2Fhow-to-use-contexts-in-go&amp;rut=def456" class='result-link'>How To Use Contexts in Go - DigitalOcean</a>
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td class='result-snippet'>
      By using the context.Context interface in the context package.
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td>
      <span class='link-text'>www.digitalocean.com/community/tutorials/how-to-use-contexts-in-go</span>
    </td>
  </tr>
  <tr><td>&nbsp;</td><td>&nbsp;</td></tr>

  <tr>
    <td valign="top">3.&nbsp;</td>
    <td>
      <a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fmedium.com%2F%40jamal%2Fcontext-guide&amp;rut=ghi789" class='result-link'>Complete Guide to Context in Golang</a>
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td class='result-snippet'>
      The Complete Guide to Context in Golang: Efficient Concurrency Management.
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td>
      <span class='link-text'>medium.com/@jamal/context-guide</span>
    </td>
  </tr>
</table>
`

const ddgLiteSponsoredAndOrganic = `
<table border="0">
  <tr class="result-sponsored">
    <td valign="top">1.&nbsp;</td>
    <td>
      <a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.udemy.com%2Fgo-course&amp;rut=xyz" class='result-link'>Golang Online Course | Udemy</a>
    </td>
  </tr>
  <tr class="result-sponsored">
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td class='result-snippet'>
      Learn Go programming from scratch.
    </td>
  </tr>
  <tr class="result-sponsored">
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td><span class='link-text'>udemy.com</span></td>
  </tr>
  <tr><td>&nbsp;</td><td>&nbsp;</td></tr>

  <tr>
    <td valign="top">2.&nbsp;</td>
    <td>
      <a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev&amp;rut=aaa" class='result-link'>The Go Programming Language</a>
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td class='result-snippet'>
      Go is an open source programming language.
    </td>
  </tr>
  <tr>
    <td>&nbsp;&nbsp;&nbsp;</td>
    <td><span class='link-text'>go.dev</span></td>
  </tr>
</table>
`

func TestParseDDGResults(t *testing.T) {
	results := parseDDGResults(ddgLiteSample, 10)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Result 1
	if results[0].Title != "context package - context - Go Packages" {
		t.Errorf("result[0].Title = %q", results[0].Title)
	}
	if results[0].Link != "https://pkg.go.dev/context" {
		t.Errorf("result[0].Link = %q", results[0].Link)
	}
	if results[0].Snippet == "" {
		t.Error("result[0].Snippet is empty")
	}

	// Result 2
	if results[1].Link != "https://www.digitalocean.com/community/tutorials/how-to-use-contexts-in-go" {
		t.Errorf("result[1].Link = %q", results[1].Link)
	}

	// Result 3 — URL-encoded path
	if results[2].Link != "https://medium.com/@jamal/context-guide" {
		t.Errorf("result[2].Link = %q", results[2].Link)
	}
}

func TestParseDDGResultsMaxCount(t *testing.T) {
	results := parseDDGResults(ddgLiteSample, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (max), got %d", len(results))
	}
}

func TestParseDDGResultsSkipsSponsored(t *testing.T) {
	results := parseDDGResults(ddgLiteSponsoredAndOrganic, 10)

	if len(results) != 1 {
		t.Fatalf("expected 1 organic result (skip sponsored), got %d", len(results))
	}
	if results[0].Title != "The Go Programming Language" {
		t.Errorf("expected organic result, got %q", results[0].Title)
	}
	if results[0].Link != "https://go.dev" {
		t.Errorf("result.Link = %q", results[0].Link)
	}
}

func TestParseDDGResultsEmpty(t *testing.T) {
	emptyPage := `
	<html><body>
	<table border="0"></table>
	<p>No results.</p>
	</body></html>
	`
	results := parseDDGResults(emptyPage, 10)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestParseDDGResultsCaptcha(t *testing.T) {
	captchaPage := `
	<html><body>
	<p>If you are not a robot, please click below to continue.</p>
	<form action="/challenge">
		<input type="hidden" name="captcha" value="1">
		<input type="submit" value="Continue">
	</form>
	</body></html>
	`
	if !isDDGCaptcha(captchaPage) {
		t.Error("expected captcha page to be detected")
	}

	normalPage := `<html><body><table><tr><td>Normal page</td></tr></table></body></html>`
	if isDDGCaptcha(normalPage) {
		t.Error("normal page should not be detected as captcha")
	}
}

func TestDDGURLDecode(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "standard redirect",
			raw:  "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath&rut=abc",
			want: "https://example.com/path",
		},
		{
			name: "already decoded",
			raw:  "https://example.com/direct",
			want: "https://example.com/direct",
		},
		{
			name: "empty",
			raw:  "",
			want: "",
		},
		{
			name: "double encoded path",
			raw:  "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath%3Fq%3Dhello%26lang%3Den&rut=xyz",
			want: "https://example.com/path?q=hello&lang=en",
		},
		{
			name: "no uddg param",
			raw:  "//duckduckgo.com/l/?rut=abc",
			want: "https://duckduckgo.com/l/?rut=abc",
		},
		{
			name: "protocol-relative without uddg",
			raw:  "//example.com/page",
			want: "https://example.com/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeDDGURL(tt.raw)
			if got != tt.want {
				t.Errorf("decodeDDGURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestWebSearchLive makes a real HTTP request to DDG lite and verifies parsing.
// Skipped in CI (set RUN_LIVE_TESTS=1 to enable).
func TestWebSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", "https://lite.duckduckgo.com/lite/?q=golang+context&kl=us-en", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AgentsGo/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	t.Logf("Response length: %d bytes", len(body))

	if isDDGCaptcha(html) {
		t.Skip("rate limited by DDG, skipping")
	}

	results := parseDDGResults(html, 5)
	t.Logf("Parsed %d results", len(results))
	for i, r := range results {
		t.Logf("  %d. %s\n     %s\n     %s", i+1, r.Title, r.Link, r.Snippet)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result from live DDG query")
	}
	for i, r := range results {
		if r.Title == "" {
			t.Errorf("result[%d] has empty title", i)
		}
		if r.Link == "" || r.Link == "https:" {
			t.Errorf("result[%d] has invalid link: %q", i, r.Link)
		}
	}
}

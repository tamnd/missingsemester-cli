// Package missingsemester is the library behind the missing command line:
// the HTTP client, request shaping, and the typed data models for the
// Missing Semester of Your CS Education (missing.csail.mit.edu).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public site throws under load.
package missingsemester

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to missing.csail.mit.edu.
const DefaultUserAgent = "missing/dev (+https://github.com/tamnd/missingsemester-cli)"

// Host is the site this client talks to.
const Host = "missing.csail.mit.edu"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Client talks to missing.csail.mit.edu over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Retries:   3,
	}
}

var (
	// liRe matches lecture list items in a year index page.
	// Captures: date (optional), year, slug, title.
	liRe = regexp.MustCompile(
		`(?s)<li>\s*(?:<strong>([^<]*)</strong>:)?\s*<a href="/(\d+)/([\w-]+)/">([^<]+)</a>`)

	// titleRe matches the <h1 class="title"> on a lecture page.
	titleRe = regexp.MustCompile(`<h1[^>]*class="title"[^>]*>([^<]+)</h1>`)

	// videoRe extracts a YouTube video embed ID.
	videoRe = regexp.MustCompile(`youtube\.com/embed/([\w-]+)`)

	// yearLinkRe matches year index links in /past/ page.
	yearLinkRe = regexp.MustCompile(`href="/(20\d\d)/"`)

	// tagRE strips HTML tags.
	tagRE = regexp.MustCompile(`<[^>]+>`)

	// contentStartRe finds the opening of the content div.
	contentStartRe = regexp.MustCompile(`<div[^>]+id="content"[^>]*>`)
)

// Lectures fetches the lecture list for a given year.
func (c *Client) Lectures(ctx context.Context, year int) ([]*Lecture, error) {
	body, err := c.Get(ctx, fmt.Sprintf("%s/%d/", BaseURL, year))
	if err != nil {
		return nil, err
	}
	html := string(body)

	var lectures []*Lecture
	rank := 1
	for _, m := range liRe.FindAllStringSubmatch(html, -1) {
		date := strings.TrimSpace(m[1])
		y, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		slug := m[3]
		title := strings.TrimSpace(m[4])
		lectures = append(lectures, &Lecture{
			Rank:  rank,
			Year:  y,
			Slug:  slug,
			Date:  date,
			Title: title,
			URL:   fmt.Sprintf("%s/%d/%s/", BaseURL, y, slug),
		})
		rank++
	}
	if len(lectures) == 0 {
		return nil, fmt.Errorf("no lectures found for year %d", year)
	}
	return lectures, nil
}

// LectureDetail fetches one lecture's full content.
func (c *Client) LectureDetail(ctx context.Context, year int, slug string) (*LectureDetail, error) {
	url := fmt.Sprintf("%s/%d/%s/", BaseURL, year, slug)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	html := string(body)

	title := ""
	if m := titleRe.FindStringSubmatch(html); m != nil {
		title = strings.TrimSpace(m[1])
	}

	video := ""
	if m := videoRe.FindStringSubmatch(html); m != nil {
		video = m[1]
	}

	text := extractContent(html)

	return &LectureDetail{
		Year:  year,
		Slug:  slug,
		Title: title,
		URL:   url,
		Video: video,
		Text:  text,
	}, nil
}

// Years fetches all available course years from the site.
func (c *Client) Years(ctx context.Context) ([]*Year, error) {
	seen := map[int]bool{}
	var yearNums []int

	// Fetch /past/ to find prior year links.
	pastBody, err := c.Get(ctx, BaseURL+"/past/")
	if err == nil {
		for _, m := range yearLinkRe.FindAllStringSubmatch(string(pastBody), -1) {
			y, _ := strconv.Atoi(m[1])
			if !seen[y] {
				seen[y] = true
				yearNums = append(yearNums, y)
			}
		}
	}

	// Fetch homepage to pick up current year link too.
	homeBody, err := c.Get(ctx, BaseURL+"/")
	if err == nil {
		for _, m := range yearLinkRe.FindAllStringSubmatch(string(homeBody), -1) {
			y, _ := strconv.Atoi(m[1])
			if !seen[y] {
				seen[y] = true
				yearNums = append(yearNums, y)
			}
		}
	}

	if len(yearNums) == 0 {
		// Fallback to known years.
		yearNums = []int{2019, 2020, 2026}
		for _, y := range yearNums {
			seen[y] = true
		}
	}

	// Sort ascending.
	for i := 0; i < len(yearNums); i++ {
		for j := i + 1; j < len(yearNums); j++ {
			if yearNums[i] > yearNums[j] {
				yearNums[i], yearNums[j] = yearNums[j], yearNums[i]
			}
		}
	}

	var years []*Year
	for rank, y := range yearNums {
		count := 0
		lectures, err := c.Lectures(ctx, y)
		if err == nil {
			count = len(lectures)
		}
		years = append(years, &Year{
			Rank:     rank + 1,
			Year:     y,
			Lectures: count,
			URL:      fmt.Sprintf("%s/%d/", BaseURL, y),
		})
	}
	return years, nil
}

// Get fetches url and returns the response body.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// extractContent pulls plain text from the page content div.
func extractContent(html string) string {
	loc := contentStartRe.FindStringIndex(html)
	if loc == nil {
		return cleanText(html, 5000)
	}
	start := loc[1]
	rest := html[start:]
	// Find the first closing </div> - simple heuristic for Jekyll pages.
	end := strings.Index(rest, "</div>")
	if end > 0 {
		rest = rest[:end]
	}
	return cleanText(rest, 5000)
}

func cleanText(html string, maxChars int) string {
	s := tagRE.ReplaceAllString(html, " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxChars {
		s = s[:maxChars]
	}
	return s
}

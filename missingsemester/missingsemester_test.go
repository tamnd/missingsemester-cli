package missingsemester

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(ts *httptest.Server) *Client {
	c := NewClient()
	c.Rate = 0
	c.HTTP = &http.Client{
		Transport: &redirectTransport{base: ts.URL, inner: http.DefaultTransport},
	}
	return c
}

type redirectTransport struct {
	base  string
	inner http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := t.base + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return t.inner.RoundTrip(newReq)
}

const fakeYearPage = `<html><body>
<div id="content">
<ul>
<li><strong>1/13/20</strong>: <a href="/2020/course-shell/">Course Overview + The Shell</a></li>
<li><strong>1/14/20</strong>: <a href="/2020/shell-tools/">Shell Tools and Scripting</a></li>
<li><a href="/2020/editors/">Editors (Vim)</a></li>
</ul>
</div>
</body></html>`

func TestLectures(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fakeYearPage)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	lectures, err := c.Lectures(context.Background(), 2020)
	if err != nil {
		t.Fatal(err)
	}
	if len(lectures) != 3 {
		t.Fatalf("want 3 lectures, got %d", len(lectures))
	}
	if lectures[0].Slug != "course-shell" {
		t.Errorf("slug[0] = %q", lectures[0].Slug)
	}
	if lectures[0].Date != "1/13/20" {
		t.Errorf("date[0] = %q", lectures[0].Date)
	}
	if lectures[0].Title != "Course Overview + The Shell" {
		t.Errorf("title[0] = %q", lectures[0].Title)
	}
	if lectures[2].Date != "" {
		t.Errorf("date[2] should be empty, got %q", lectures[2].Date)
	}
	if lectures[0].Year != 2020 {
		t.Errorf("year[0] = %d", lectures[0].Year)
	}
}

const fakeLecturePage = `<html><body>
<div id="content">
<h1 class="title">Editors (Vim)</h1>
<p>A good programmer knows their editor well.</p>
<iframe src="https://www.youtube.com/embed/a8sfistAqg4" frameborder="0"></iframe>
</div>
</body></html>`

func TestLectureDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fakeLecturePage)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	detail, err := c.LectureDetail(context.Background(), 2020, "editors")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Editors (Vim)" {
		t.Errorf("title = %q", detail.Title)
	}
	if detail.Video != "a8sfistAqg4" {
		t.Errorf("video = %q", detail.Video)
	}
	if detail.Slug != "editors" {
		t.Errorf("slug = %q", detail.Slug)
	}
	if detail.Year != 2020 {
		t.Errorf("year = %d", detail.Year)
	}
}

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

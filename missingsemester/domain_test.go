package missingsemester

import (
	"testing"
)

// These tests exercise the domain info without needing a network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "missingsemester" {
		t.Errorf("Scheme = %q, want missingsemester", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "missing" {
		t.Errorf("Identity.Binary = %q, want missing", info.Identity.Binary)
	}
}

func TestExtractContent(t *testing.T) {
	html := `<html><body>
<div id="content">
<h1 class="title">Editors (Vim)</h1>
<p>A good programmer knows their editor well.</p>
</div>
</body></html>`
	text := extractContent(html)
	if text == "" {
		t.Error("extractContent returned empty string")
	}
	if len(text) < 10 {
		t.Errorf("extractContent text too short: %q", text)
	}
}

func TestCleanText(t *testing.T) {
	html := `<p>Hello <strong>world</strong></p>`
	got := cleanText(html, 1000)
	if got != "Hello world" {
		t.Errorf("cleanText = %q, want %q", got, "Hello world")
	}
}

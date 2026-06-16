package missingsemester

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes missingsemester as a kit Domain.
func init() { kit.Register(Domain{}) }

// Domain is the missingsemester driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "missingsemester",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "missing",
			Short:  "Browse the Missing Semester of Your CS Education from the command line",
			Long: `Browse the Missing Semester of Your CS Education from the command line.

missing reads public data from missing.csail.mit.edu over plain HTTPS, shapes it
into clean records, and prints output that pipes into the rest of your tools. No API
key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/missingsemester-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// list: lectures for a year
	kit.Handle(app, kit.OpMeta{Name: "list", Group: "read", List: true,
		Summary: "List lectures for a year (default: 2020)"}, listLectures)

	// lecture: one lecture detail
	kit.Handle(app, kit.OpMeta{Name: "lecture", Group: "read", Single: true,
		Summary: "Show one lecture's content",
		Args: []kit.Arg{
			{Name: "year", Help: "course year (2019, 2020, 2026)"},
			{Name: "slug", Help: "lecture slug (e.g. editors)"},
		}}, getLecture)

	// years: available course years
	kit.Handle(app, kit.OpMeta{Name: "years", Group: "read", List: true,
		Summary: "List all available course years"}, listYears)

	// export: JSONL of all lectures
	kit.Handle(app, kit.OpMeta{Name: "export", Group: "read", List: true,
		Summary: "Export lectures as JSONL"}, exportLectures)

	// info: site stats
	kit.Handle(app, kit.OpMeta{Name: "info", Group: "read", Single: true,
		Summary: "Site stats"}, siteInfo)
}

func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- input structs ---

type listIn struct {
	Year   int     `kit:"flag" default:"2020" help:"course year (2019, 2020, 2026)"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

type lectureIn struct {
	Year     string  `kit:"arg" help:"course year"`
	Slug     string  `kit:"arg" help:"lecture slug"`
	MaxChars int     `kit:"flag" default:"5000" help:"truncate body text"`
	Client   *Client `kit:"inject"`
}

type yearsIn struct {
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

type exportIn struct {
	Year   int     `kit:"flag" default:"2020" help:"year to export (0 = all years)"`
	Client *Client `kit:"inject"`
}

type infoIn struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listLectures(ctx context.Context, in listIn, emit func(*Lecture) error) error {
	lectures, err := in.Client.Lectures(ctx, in.Year)
	if err != nil {
		return mapErr(err)
	}
	for i, l := range lectures {
		if in.Limit > 0 && i >= in.Limit {
			break
		}
		if err := emit(l); err != nil {
			return err
		}
	}
	return nil
}

func getLecture(ctx context.Context, in lectureIn, emit func(*LectureDetail) error) error {
	year, err := strconv.Atoi(in.Year)
	if err != nil {
		return errs.Usage("year must be a number: %s", in.Year)
	}
	detail, err := in.Client.LectureDetail(ctx, year, in.Slug)
	if err != nil {
		return mapErr(err)
	}
	if in.MaxChars > 0 && len(detail.Text) > in.MaxChars {
		detail.Text = detail.Text[:in.MaxChars]
	}
	return emit(detail)
}

func listYears(ctx context.Context, in yearsIn, emit func(*Year) error) error {
	years, err := in.Client.Years(ctx)
	if err != nil {
		return mapErr(err)
	}
	for i, y := range years {
		if in.Limit > 0 && i >= in.Limit {
			break
		}
		if err := emit(y); err != nil {
			return err
		}
	}
	return nil
}

func exportLectures(ctx context.Context, in exportIn, emit func(*LectureDetail) error) error {
	var yearNums []int
	if in.Year == 0 {
		years, err := in.Client.Years(ctx)
		if err != nil {
			return mapErr(err)
		}
		for _, y := range years {
			yearNums = append(yearNums, y.Year)
		}
	} else {
		yearNums = []int{in.Year}
	}

	for _, y := range yearNums {
		lectures, err := in.Client.Lectures(ctx, y)
		if err != nil {
			return mapErr(err)
		}
		for _, l := range lectures {
			detail, err := in.Client.LectureDetail(ctx, l.Year, l.Slug)
			if err != nil {
				return mapErr(err)
			}
			if err := emit(detail); err != nil {
				return err
			}
		}
	}
	return nil
}

func siteInfo(ctx context.Context, in infoIn, emit func(*Info) error) error {
	years, err := in.Client.Years(ctx)
	if err != nil {
		// Return static info if we can't fetch.
		return emit(&Info{
			Site:     "The Missing Semester of Your CS Education",
			Host:     Host,
			Years:    3,
			Lectures: 28,
			Source:   "MIT CSAIL",
		})
	}
	total := 0
	for _, y := range years {
		total += y.Lectures
	}
	return emit(&Info{
		Site:     "The Missing Semester of Your CS Education",
		Host:     Host,
		Years:    len(years),
		Lectures: total,
		Source:   "MIT CSAIL",
	})
}

// --- Resolver (domain.go interface) ---

func (Domain) Classify(input string) (uriType, id string, err error) {
	return "", "", fmt.Errorf("missingsemester URIs not supported for pasted URLs")
}

func (Domain) Locate(uriType, id string) (string, error) {
	return "", errs.Usage("missingsemester has no URI type %q", uriType)
}

func mapErr(err error) error {
	return err
}

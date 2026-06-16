package missingsemester

// Lecture is one lecture in the Missing Semester course.
type Lecture struct {
	Rank  int    `json:"rank"  csv:"rank"  tsv:"rank"`
	Year  int    `json:"year"  csv:"year"  tsv:"year"`
	Slug  string `json:"slug"  csv:"slug"  tsv:"slug"`
	Date  string `json:"date"  csv:"date"  tsv:"date"`
	Title string `json:"title" csv:"title" tsv:"title"`
	URL   string `json:"url"   csv:"url"   tsv:"url"`
}

// LectureDetail is one lecture with full body text.
type LectureDetail struct {
	Year  int    `json:"year"  csv:"year"  tsv:"year"`
	Slug  string `json:"slug"  csv:"slug"  tsv:"slug"`
	Title string `json:"title" csv:"title" tsv:"title"`
	URL   string `json:"url"   csv:"url"   tsv:"url"`
	Video string `json:"video" csv:"video" tsv:"video"`
	Text  string `json:"text"  csv:"text"  tsv:"text"`
}

// Year is one course offering.
type Year struct {
	Rank     int    `json:"rank"     csv:"rank"     tsv:"rank"`
	Year     int    `json:"year"     csv:"year"     tsv:"year"`
	Lectures int    `json:"lectures" csv:"lectures" tsv:"lectures"`
	URL      string `json:"url"      csv:"url"      tsv:"url"`
}

// Info is site-level stats.
type Info struct {
	Site     string `json:"site"     csv:"site"     tsv:"site"`
	Host     string `json:"host"     csv:"host"     tsv:"host"`
	Years    int    `json:"years"    csv:"years"    tsv:"years"`
	Lectures int    `json:"lectures" csv:"lectures" tsv:"lectures"`
	Source   string `json:"source"   csv:"source"   tsv:"source"`
}

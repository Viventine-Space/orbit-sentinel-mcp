package main

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"shorter than limit", "SpaceX", 20, "SpaceX"},
		{"exactly at limit", "SpaceX", 6, "SpaceX"},
		{"one over limit", "SpaceXX", 6, "Spa..."},
		{"ascii truncated", "Application for modification", 12, "Applicati..."},
		{"multi-byte cut on boundary", "中国卫星通信集团有限公司", 20, "中国卫星通..."},
		{"multi-byte limit lands mid-rune", "北京航天", 8, "北..."},
		{"multi-byte fits", "北京航天", 12, "北京航天"},
		{"limit below ellipsis", "SpaceX", 2, "Sp"},
		{"limit below ellipsis multi-byte", "北京", 2, ""},
		{"zero limit", "SpaceX", 0, ""},
		{"negative limit", "SpaceX", -1, ""},
		{"pipe escaped", "A|B", 20, "A\\|B"},
		{"pipe escaped after cut", "Order|Grant|Denial", 12, "Order\\|Gra..."},
		{"newline collapsed", "line one\nline two", 40, "line one line two"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.in, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncate(%q, %d) = %q, not valid UTF-8", tt.in, tt.maxLen, got)
			}
		})
	}
}

// Titles reach the table formatters as truncate(deref(x), n); escaping must not
// compound across the two hops.
func TestTruncateAfterDerefEscapesOnce(t *testing.T) {
	title := "Application of A|B Satellite for\nmodification of authority"
	got := truncate(deref(&title), 40)
	if strings.Contains(got, "\\\\|") {
		t.Errorf("double-escaped pipe in %q", got)
	}
	if want := "Application of A\\|B Satellite for mod..."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMdCell(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "Intelsat License LLC", "Intelsat License LLC"},
		{"single pipe", "Grant|Deny", "Grant\\|Deny"},
		{"multiple pipes", "a|b|c", "a\\|b\\|c"},
		{"already escaped", "a\\|b", "a\\|b"},
		{"newline", "first\nsecond", "first second"},
		{"crlf", "first\r\nsecond", "first second"},
		{"tab", "first\tsecond", "first second"},
		{"multi-byte preserved", "中国卫星|通信", "中国卫星\\|通信"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mdCell(tt.in)
			if got != tt.want {
				t.Errorf("mdCell(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if again := mdCell(got); again != got {
				t.Errorf("mdCell not idempotent: %q -> %q", got, again)
			}
		})
	}
}

func TestFormatUSD(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{5, "5"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1000000, "1,000,000"},
		{3000000000, "3,000,000,000"},
	}
	for _, tt := range tests {
		got := formatUSD(tt.in)
		if got != tt.want {
			t.Errorf("formatUSD(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatUSDPtr(t *testing.T) {
	if got := formatUSDPtr(nil); got != "-" {
		t.Errorf("formatUSDPtr(nil) = %q, want -", got)
	}
	v := int64(500000)
	if got := formatUSDPtr(&v); got != "$500,000" {
		t.Errorf("formatUSDPtr(500000) = %q, want $500,000", got)
	}
}

func TestExtractFilingKeywords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"strips stop words", "What are the recent FCC filings about spectrum sharing?", "spectrum sharing"},
		{"strips agencies and generic terms", "Show me the top FCC filers for debris mitigation", "debris mitigation"},
		{"drops bare numbers", "filings from 2024 about debris mitigation", "debris mitigation"},
		{"all stop words", "what are the most recent filings?", ""},
		{"empty question", "", ""},
		{"keeps proper nouns", "SpaceX Starlink Gen2 modification", "spacex starlink gen2 modification"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractFilingKeywords(tt.in); got != tt.want {
				t.Errorf("extractFilingKeywords(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractEntityNames(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"known operator", "How many filings does SpaceX have?", "spacex"},
		{"multiple known operators", "Compare Viasat and Telesat filings", "viasat telesat"},
		{"short term needs word boundary", "What is SES filing about?", "ses"},
		{"short term substring ignored", "sessions about spectrum", ""},
		{"acronym fallback", "filings mentioning NGSO coordination", "NGSO"},
		{"skips agency acronyms", "recent FCC activity", ""},
		{"empty question", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractEntityNames(tt.in); got != tt.want {
				t.Errorf("extractEntityNames(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// formatEntity renders filing_stats from a Go map; the agency rows must come
// out in a stable order rather than the map's randomized iteration order.
func TestFormatEntityStableAndEscaped(t *testing.T) {
	input := []byte(`{
		"id": "3f2b1c4d-0000-0000-0000-000000000001",
		"canonical_name": "Example|Sat Holdings",
		"entity_type": "operator",
		"country": "US",
		"fcc_frn": "0001234567",
		"filing_count": 7,
		"filing_stats": {"ITU": 2, "FCC": 4, "FAA_AST": 1},
		"earliest_filing": "2019-03-01",
		"latest_filing": "2026-01-15",
		"satellites": [
			{"name": "EXAMPLE|SAT-1", "norad_cat_id": 44713, "cospar_id": "2019-074A", "orbit_class": "LEO", "orbital_status": "active"}
		]
	}`)

	want := "# Entity: Example|Sat Holdings\n\n" +
		"**Type:** operator | **Country:** US\n" +
		"**Identifiers:** IBFS FRN: 0001234567\n" +
		"**Total Filings:** 7 | **Active:** 2019-03-01 to 2026-01-15\n" +
		"\n## Filing Breakdown\n" +
		"| Agency | Count |\n" +
		"|--------|-------|\n" +
		"| FAA_AST | 1 |\n" +
		"| FCC | 4 |\n" +
		"| ITU | 2 |\n" +
		"\n## Satellites\n" +
		"| Name | NORAD ID | COSPAR | Orbit | Status |\n" +
		"|------|----------|--------|-------|--------|\n" +
		"| EXAMPLE\\|SAT-1 | 44713 | 2019-074A | LEO | active |\n" +
		"\n---\n_Source: Orbit Sentinel database. Verify claims against the source document URL above when available._\n"

	got := formatEntity(input)
	if got != want {
		t.Errorf("formatEntity output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}

	for i := 0; i < 20; i++ {
		if again := formatEntity(input); again != got {
			t.Fatalf("formatEntity is not deterministic across calls:\n%s", again)
		}
	}
}

// The trends header joins optional filters; with no agency or entity filter it
// must not lead with a dangling separator.
func TestFormatTrendsHeader(t *testing.T) {
	tests := []struct {
		name    string
		filters trendFilters
		want    string
	}{
		{"no filters", trendFilters{Periods: 4, PeriodMonths: 3}, "**4 periods of 3 months**\n"},
		{"agency only", trendFilters{Agency: "FCC", Periods: 4, PeriodMonths: 3}, "**Agency:** FCC | **4 periods of 3 months**\n"},
		{
			"agency and entity",
			trendFilters{Agency: "FCC", EntityID: "abc-123", Periods: 2, PeriodMonths: 6},
			"**Agency:** FCC | **Entity:** abc-123 | **2 periods of 6 months**\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(trendsResponse{Filters: tt.filters})
			if err != nil {
				t.Fatal(err)
			}
			got := formatTrends(data)
			header := strings.TrimPrefix(got, "# Filing Trends\n\n")
			header, _, _ = strings.Cut(header, "\n\n")
			if header+"\n" != tt.want {
				t.Errorf("header = %q, want %q", header+"\n", tt.want)
			}
		})
	}
}

func TestFormatTrends(t *testing.T) {
	delta := 12
	pct := 31.5
	moverPct := -8.25
	data, err := json.Marshal(trendsResponse{
		Periods: []trendPeriod{
			{PeriodStart: "2025-01-01", PeriodEnd: "2025-03-31", FilingCount: 38},
			{PeriodStart: "2025-04-01", PeriodEnd: "2025-06-30", FilingCount: 50, Delta: &delta, PctChange: &pct},
		},
		TopMovers: []trendMover{
			{EntityID: "e1", CanonicalName: "Alpha|Space Inc", CurrentCount: 11, PreviousCount: 12, Delta: -1, PctChange: &moverPct},
		},
		Filters: trendFilters{Periods: 2, PeriodMonths: 3},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "# Filing Trends\n\n" +
		"**2 periods of 3 months**\n\n" +
		"| Period | Filings | Delta | Change |\n" +
		"|--------|---------|-------|--------|\n" +
		"| 2025-01-01 to 2025-03-31 | 38 | - | - |\n" +
		"| 2025-04-01 to 2025-06-30 | 50 | +12 | +31.5% |\n" +
		"\n## Top Movers\n" +
		"| Entity | Current | Previous | Delta | Change |\n" +
		"|--------|---------|----------|-------|--------|\n" +
		"| Alpha\\|Space Inc | 11 | 12 | -1 | -8.2% |\n" +
		"\n_Source: Orbit Sentinel database._\n"

	if got := formatTrends(data); got != want {
		t.Errorf("formatTrends output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Every cell a formatter emits must keep its row's pipe count intact, otherwise
// the Markdown table silently gains columns.
func TestFormatFilingListEscapesCells(t *testing.T) {
	data := []byte(`{
		"data": [
			{"id": "f1", "source_agency": "FCC", "filing_type": "SATELLITE_LICENSE",
			 "title": "Application of A|B Corp\nfor modification", "filed_date": "2025-07-01",
			 "applicant_name": "A|B Corp"}
		],
		"pagination": {"page": 1, "per_page": 10, "total": 1, "total_pages": 1}
	}`)

	got := formatFilingList(data)
	var row string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "| 1 |") {
			row = line
		}
	}
	if row == "" {
		t.Fatalf("no data row in output:\n%s", got)
	}
	if strings.Count(row, "\n") != 0 {
		t.Errorf("row contains a newline: %q", row)
	}
	if want := "| 1 | f1 | FCC | SATELLITE_LICENSE | Application of A\\|B Corp for modification | 2025-07-01 | A\\|B Corp |"; row != want {
		t.Errorf("row = %q, want %q", row, want)
	}
}

func TestFormatLaunchHistoryRendersLaunchTag(t *testing.T) {
	data := []byte(`{
		"data": [
			{"operation_date": "2024-03-04", "vehicle_type": "Falcon 9", "launch_site": "CCSFS SLC-40",
			 "mission_name": "Starlink Group 6-41", "outcome": "success", "launch_tag": "2024-045"},
			{"operation_date": "2024-01-03", "vehicle_type": "Electron", "launch_site": "Mahia LC-1",
			 "mission_name": "Four Of A Kind", "outcome": "success"}
		],
		"total": 2
	}`)

	want := "# Launch History (2 operations)\n\n" +
		"| Date | Vehicle | Site | Mission | Outcome | Launch Tag |\n" +
		"|------|---------|------|---------|---------|------------|\n" +
		"| 2024-03-04 | Falcon 9 | CCSFS SLC-40 | Starlink Group 6-41 | success | 2024-045 |\n" +
		"| 2024-01-03 | Electron | Mahia LC-1 | Four Of A Kind | success | - |\n"

	if got := formatLaunchHistory(data); got != want {
		t.Errorf("formatLaunchHistory output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

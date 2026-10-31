package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newTestSession connects an in-memory MCP client to a server whose REST API
// is api.
func newTestSession(t *testing.T, api http.Handler) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	server := newServer(&APIClient{BaseURL: srv.URL, HTTPClient: srv.Client()})
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callText(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func jsonAPI(body any) http.Handler {
	b, _ := json.Marshal(body)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})
}

// secRows is the shape of the 708 KB response: 500 8-Ks with 1,000-character
// excerpts (BUGLIST day-3 #21).
func secRows(n int) map[string]any {
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{
			"form_type": "8-K", "filed_date": "2025-03-01", "accession_number": fmt.Sprintf("0001-25-%06d", i),
			"filing_url": "https://www.sec.gov/Archives/edgar/data/1/000125" + strconv.Itoa(i) + ".htm",
			"items":      []string{"7.01", "9.01"}, "text_excerpt": strings.Repeat("Material event. ", 63),
			"company": "Example Space Corp", "ticker": "EXSP", "cik": "1234567",
		}
	}
	return map[string]any{"results": rows, "total": 1234}
}

func satRows(n int) map[string]any {
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{
			"name": fmt.Sprintf("STARLINK-%d", i), "norad_cat_id": 44000 + i, "cospar_id": "2019-029A",
			"operator_name": "SpaceX", "operator_country": "US", "orbit_class": "LEO", "object_type": "PAYLOAD",
			"orbital_status": "ON_ORBIT", "launch_date": "2019-05-24", "entity_id": "0b8f5d1e-6a0c-4f7e-9d3c-1a2b3c4d5e6f",
			"purpose": strings.Repeat("Communications ", 20),
		}
	}
	return map[string]any{"results": rows, "total": 9000}
}

// cellSep matches a column separator: a pipe not escaped by a backslash.
var cellSep = regexp.MustCompile(`(^|[^\\])\|`)

var omittedRe = regexp.MustCompile(`(\d+) rows omitted`)

// An oversize list keeps the total on its first line, stays under the cap and
// says how many rows it left out; shown plus omitted is every returned row.
func TestOversizeListIsCappedWithTotalFirst(t *testing.T) {
	for _, tc := range []struct {
		tool  string
		body  map[string]any
		total string
		idRe  string
	}{
		{"search_sec_filings", secRows(500), "1234", `0001-25-\d{6}`},
		{"search_satellites", satRows(500), "9000", `STARLINK-\d+`},
	} {
		text := callText(t, newTestSession(t, jsonAPI(tc.body)), tc.tool, map[string]any{"limit": 500})
		if len(text) > maxToolOutputBytes {
			t.Errorf("%s: %d bytes, want <= %d", tc.tool, len(text), maxToolOutputBytes)
		}
		first, _, _ := strings.Cut(text, "\n")
		if !strings.Contains(first, tc.total) {
			t.Errorf("%s: first line %q does not carry total %s", tc.tool, first, tc.total)
		}
		m := omittedRe.FindStringSubmatch(text)
		if m == nil {
			t.Fatalf("%s: no omitted-rows note in output ending %q", tc.tool, text[max(0, len(text)-200):])
		}
		omitted, _ := strconv.Atoi(m[1])
		shown := len(regexp.MustCompile(tc.idRe).FindAllString(text, -1))
		if shown+omitted != 500 || shown < 100 {
			t.Errorf("%s: shown %d + omitted %d, want 500 with at least 100 shown", tc.tool, shown, omitted)
		}
	}
}

// Every tool's output passes the same final guard, including the custom
// formatters (here a filing with a 1 MB summary).
func TestEveryToolOutputIsCapped(t *testing.T) {
	big := strings.Repeat("x", 1<<20)
	text := callText(t, newTestSession(t, jsonAPI(map[string]any{
		"id": "f1", "source_agency": "FCC", "source_id": "SAT-LOA-1", "filing_type": "OTHER",
		"status": "FILED", "extraction_status": "completed", "summary": big,
	})), "get_filing_detail", map[string]any{"id": "f1"})
	if len(text) > maxToolOutputBytes || !strings.Contains(text, "truncated") {
		t.Fatalf("filing detail: %d bytes, want <= %d with a truncation note", len(text), maxToolOutputBytes)
	}
}

func TestFormatRowsSmallCountAndOtherShapes(t *testing.T) {
	small := formatRows(json.RawMessage(`{"results":[{"a":1},{"a":2}],"total":2}`), nil)
	if !strings.HasPrefix(small, "Total matches: 2. Showing 2 rows.\n") || strings.Contains(small, "omitted") ||
		!strings.Contains(small, `{"a":1}`+"\n"+`{"a":2}`) {
		t.Errorf("small list:\n%s", small)
	}
	count := formatRows(json.RawMessage(`{"results":[],"total":12345}`), satelliteTable)
	if count != "Total matches: 12345. No rows returned.\n" {
		t.Errorf("count_only: %q", count)
	}
	ms := formatRows(json.RawMessage(`{"count":1,"milestones":[{"call_sign":"S1"}],"total":40}`), nil)
	if !strings.HasPrefix(ms, "Total matches: 40. Showing 1 rows.\ncount: 1\n") || !strings.Contains(ms, `"call_sign":"S1"`) {
		t.Errorf("milestones:\n%s", ms)
	}
	gs := formatRows(json.RawMessage(`{"groups":[{"key":"HI","call_signs":62}],"group_by":"state","source":"fcc_ibfs","total":62}`), groundStationTable)
	if !strings.Contains(gs, `group_by: "state"`) || !strings.Contains(gs, `{"key":"HI","call_signs":62}`) {
		t.Errorf("groups:\n%s", gs)
	}
	if got := formatRows(json.RawMessage(`{"entity":{"id":"e1"}}`), nil); got != formatJSON(json.RawMessage(`{"entity":{"id":"e1"}}`)) {
		t.Errorf("non-list response must pretty-print as before: %q", got)
	}
}

func TestRowTablesKeepIDs(t *testing.T) {
	cases := []struct {
		name string
		t    *rowTable
		row  string
		want []string
	}{
		{"sec", secTable, `{"form_type":"8-K","filed_date":"2025-03-01","accession_number":"0001-25-000001","filing_url":"https://sec.gov/x","items":["7.01"],"text_excerpt":"Line one|two","company":"AST SpaceMobile","ticker":"ASTS","cik":"1780312","entity_id":"e-1"}`,
			[]string{"| 2025-03-01 | - | 8-K | AST SpaceMobile | ASTS | 1780312 | 7.01 | 0001-25-000001 | https://sec.gov/x | e-1 | Line one\\|two |"}},
		{"satellite", satelliteTable, `{"name":"STARLINK-1007","norad_cat_id":44713,"cospar_id":"2019-074A","operator_name":"SpaceX","operator_country":"US","orbit_class":"LEO","perigee_km":540.4,"apogee_km":551,"inclination_deg":53.05,"object_type":"PAYLOAD","orbital_status":"ON_ORBIT","launch_date":"2019-11-11","alternate_names":["Starlink 1007"]}`,
			[]string{"| STARLINK-1007 | 44713 | 2019-074A | SpaceX | US | LEO | 540 | 551 | 53.05 | PAYLOAD | ON_ORBIT | 2019-11-11 | - | - | Starlink 1007 |"}},
		{"spectrum", spectrumTable, `{"frequency_low_mhz":11700,"frequency_high_mhz":12200,"direction":"downlink","filing_id":"f-1","source_agency":"FCC","source_id":"SAT-LOA-1","filed_date":"2020-05-26","applicant":"Space Exploration Holdings"}`,
			[]string{"| 11700 | 12200 | - | downlink | - | - | FCC | SAT-LOA-1 | - | 2020-05-26 | Space Exploration Holdings | f-1 |"}},
		{"ground station", groundStationTable, `{"source":"fcc_ibfs","station_name":"E190123","operator":"SpaceX","state":"HI","latitude":21.3069,"longitude":-157.8583,"frequency_bands":["Ka","Ku"],"distance_km":12.34}`,
			[]string{"| fcc_ibfs | E190123 | SpaceX | HI | 21.3069 | -157.8583 | 12.3 | Ka,Ku | - | - |"}},
		{"space station", spaceStationTable, `{"call_sign":"S2357","system_name":"Lightspeed","operator_name":"Telesat","licensing_admin":"Canada","is_ngso":true,"frequencies":"17.8-20.2 GHz|27.5-30 GHz","granted":true,"foreign":true,"us_market_access":true,"entity_id":"e-1"}`,
			[]string{"| S2357 | Lightspeed | Telesat | Canada | NGSO | - | 17.8-20.2 GHz\\|27.5-30 GHz | - | yes | yes | yes | e-1 |"}},
	}
	for _, c := range cases {
		got := c.t.row(0, json.RawMessage(c.row))
		for _, w := range c.want {
			if got != w {
				t.Errorf("%s row:\n got %s\nwant %s", c.name, got, w)
			}
		}
		if strings.Count(c.t.header, "\n") != 2 || len(cellSep.FindAllString(got, -1)) != strings.Count(strings.SplitN(c.t.header, "\n", 2)[0], "|") {
			t.Errorf("%s: row and header column counts differ", c.name)
		}
	}
	if secTable.row(10, json.RawMessage(`{"text_excerpt":"late"}`)) != "| - | - | - | - | - | - | - | - | - | - |  |" {
		t.Error("sec excerpt must be shown for the first 10 rows only")
	}
	long := `{"text_excerpt":"` + strings.Repeat("x", 3000) + `"}`
	if got := secTableFor(true).row(10, json.RawMessage(long)); !strings.Contains(got, strings.Repeat("x", 3000)) {
		t.Error("full_excerpt must show every row's whole excerpt")
	}
}

func TestTextResultGuard(t *testing.T) {
	r := textResult(strings.Repeat("é", 1<<19))
	text := r.Content[0].(*mcp.TextContent).Text
	if len(text) > maxToolOutputBytes || !strings.HasSuffix(text, "narrow the query._\n") || !utf8.ValidString(text) {
		t.Fatalf("guarded text: %d bytes, valid UTF-8 %v", len(text), utf8.ValidString(text))
	}
	if textResult("short").Content[0].(*mcp.TextContent).Text != "short" {
		t.Fatal("short text must pass unchanged")
	}
}

func TestToolCallLogsOutputBytes(t *testing.T) {
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(orig) })

	text := callText(t, newTestSession(t, jsonAPI(map[string]any{"results": []any{}, "total": 3})), "search_satellites", nil)
	if want := fmt.Sprintf(`"bytes":%d`, len(text)); !strings.Contains(buf.String(), want) {
		t.Fatalf("log %q lacks %s", buf.String(), want)
	}
}

// The FAA financial-responsibility table comes from the operator name as well
// as entity_id (not every FAA order is linked to an entity), and carries the
// reentry and government-property figures (SpaceBench sweep 10-31 O1, O2).
func TestBondPortfolioFAAByOperator(t *testing.T) {
	var faaQuery string
	cs := newTestSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/financial-responsibility":
			faaQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"data":[{"operator":"Varda Space Industries","license_number":"RLO-23-001","license_type":"reentry","vehicle_type":"W-Series","launch_site":"UTTR","reentry_tpl_usd":5000000,"govt_property_usd":0,"effective_date":"2024-02-01"}],"pagination":{"page":1,"per_page":100,"total":1,"total_pages":1}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	out := callText(t, cs, "get_bond_portfolio", map[string]any{"operator": "Varda"})
	if !strings.Contains(faaQuery, "operator=Varda") {
		t.Fatalf("financial-responsibility query = %q, want operator=Varda", faaQuery)
	}
	for _, want := range []string{"| Reentry TPL | Govt property |", "| reentry |", "| $5000000 | $0 | 2024-02-01 |"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// search_positions rows carry the filing id and date the footer tells the
// model to use (SpaceBench sweep 10-31 O11).
func TestPositionSearchShowsFiling(t *testing.T) {
	out := formatPositionSearch(json.RawMessage(`{"total":1,"results":[{"filing_id":"f-1","filed_date":"2025-04-02","docket_number":"25-306","argument_text":"x"}]}`))
	if !strings.Contains(out, "| f-1 | 2025-04-02 | 25-306 |") {
		t.Errorf("row lacks filing id and date:\n%s", out)
	}
}

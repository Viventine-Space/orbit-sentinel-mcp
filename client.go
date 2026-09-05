package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// APIClient wraps HTTP calls to the Orbit Sentinel REST API.
type APIClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewAPIClient creates a client from MCP_API_URL and MCP_API_KEY env vars.
func NewAPIClient() *APIClient {
	baseURL := os.Getenv("MCP_API_URL")
	if baseURL == "" {
		baseURL = "https://orbit-sentinel.viventine.com"
	}
	return &APIClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     os.Getenv("MCP_API_KEY"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// mcpToolContextKey is the key under which we stash the MCP tool name on
// the context so doGet/doPost can stamp it onto the X-MCP-Tool header.
type mcpToolContextKey struct{}

// WithMCPTool returns a context that carries the MCP tool name. Each tool
// handler wraps its incoming context with this before calling client methods,
// so every downstream REST call is attributed back to the tool that drove it.
func WithMCPTool(ctx context.Context, toolName string) context.Context {
	return context.WithValue(ctx, mcpToolContextKey{}, toolName)
}

func mcpToolFromContext(ctx context.Context) string {
	v, _ := ctx.Value(mcpToolContextKey{}).(string)
	return v
}

// apiKeyContextKey is the key under which the per-request caller API key is
// stashed on the context in HTTP mode.
type apiKeyContextKey struct{}

// WithAPIKey returns a context carrying the caller's API key. In HTTP mode the
// auth middleware stamps the key from the incoming Authorization header so each
// request forwards the caller's own key to the REST API.
func WithAPIKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, apiKeyContextKey{}, key)
}

func apiKeyFromContext(ctx context.Context) string {
	v, _ := ctx.Value(apiKeyContextKey{}).(string)
	return v
}

func (c *APIClient) doGet(ctx context.Context, path string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.applyHeaders(req, path, ctx)
	return c.do(req)
}

func (c *APIClient) doPost(ctx context.Context, path string, payload []byte) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req, path, ctx)
	return c.do(req)
}

func (c *APIClient) applyHeaders(req *http.Request, path string, ctx context.Context) {
	key := apiKeyFromContext(ctx)
	if key == "" {
		key = c.APIKey
	}
	if key != "" && strings.HasPrefix(path, "/api/") {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if tool := mcpToolFromContext(ctx); tool != "" {
		req.Header.Set("X-MCP-Tool", tool)
	}
}

func (c *APIClient) do(req *http.Request) (json.RawMessage, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s failed: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		excerpt := string(body)
		if len(excerpt) > 200 {
			excerpt = excerpt[:200]
		}
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, excerpt)
	}
	return json.RawMessage(body), nil
}

// SearchFilings calls GET /api/v1/filings with query parameters.
func (c *APIClient) SearchFilings(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/filings", params))
}

// GetFiling calls GET /api/v1/filings/{id}.
func (c *APIClient) GetFiling(ctx context.Context, id string) (json.RawMessage, error) {
	return c.doGet(ctx, "/api/v1/filings/"+url.PathEscape(id))
}

// SearchSpectrum calls GET /api/v1/spectrum.
func (c *APIClient) SearchSpectrum(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/spectrum", params))
}

// SearchSECFilings calls GET /api/v1/sec/filings.
func (c *APIClient) SearchSECFilings(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/sec/filings", params))
}

// SearchScreening calls GET /api/v1/screening.
func (c *APIClient) SearchScreening(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/screening", params))
}

// GetEntityDossier calls GET /api/v1/entities/{id}/dossier with optional params
// (include_family, family_confidence).
func (c *APIClient) GetEntityDossier(ctx context.Context, id string, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/entities/"+url.PathEscape(id)+"/dossier", params))
}

// SearchSatellites calls GET /api/v1/satellites.
func (c *APIClient) SearchSatellites(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/satellites", params))
}

// SearchGroundStations calls GET /api/v1/ground-stations.
func (c *APIClient) SearchGroundStations(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/ground-stations", params))
}

// SearchFederalAwards calls GET /api/v1/federal-awards.
func (c *APIClient) SearchFederalAwards(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/federal-awards", params))
}

// SemanticSearchParams holds parameters for semantic search.
type SemanticSearchParams struct {
	Query         string
	Limit         int
	Agency        string
	MinSimilarity float64
	FiledAfter    string
	FiledBefore   string
}

// SearchSemantic calls POST /api/v1/search/semantic.
func (c *APIClient) SearchSemantic(ctx context.Context, query string, limit int, agency string, minSimilarity float64) (json.RawMessage, error) {
	return c.SearchSemanticFull(ctx, SemanticSearchParams{
		Query: query, Limit: limit, Agency: agency, MinSimilarity: minSimilarity,
	})
}

// SearchSemanticFull calls POST /api/v1/search/semantic with full parameters.
func (c *APIClient) SearchSemanticFull(ctx context.Context, p SemanticSearchParams) (json.RawMessage, error) {
	reqBody := map[string]any{"query": p.Query}
	if p.Limit > 0 {
		reqBody["limit"] = p.Limit
	}
	if p.Agency != "" {
		reqBody["agency"] = p.Agency
	}
	if p.MinSimilarity > 0 {
		reqBody["min_similarity"] = p.MinSimilarity
	}
	if p.FiledAfter != "" {
		reqBody["filed_after"] = p.FiledAfter
	}
	if p.FiledBefore != "" {
		reqBody["filed_before"] = p.FiledBefore
	}
	payload, _ := json.Marshal(reqBody)
	return c.doPost(ctx, "/api/v1/search/semantic", payload)
}

// SearchEntities calls GET /api/v1/entities with query parameters.
func (c *APIClient) SearchEntities(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/entities", params))
}

// GetEntityProfile calls GET /api/v1/entities/{id} with optional date filters.
func (c *APIClient) GetEntityProfile(ctx context.Context, id string, params ...map[string]string) (json.RawMessage, error) {
	path := "/api/v1/entities/" + url.PathEscape(id)
	if len(params) > 0 {
		path = buildPath(path, params[0])
	}
	return c.doGet(ctx, path)
}

// SearchPositions calls GET /api/v1/positions/search with query parameters.
// Returns LLM-extracted policy arguments filtered by docket / stance / filer.
func (c *APIClient) SearchPositions(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/positions/search", params))
}

// GetStatus calls GET /status.
func (c *APIClient) GetStatus(ctx context.Context) (json.RawMessage, error) {
	return c.doGet(ctx, "/status")
}

// GetTopFilers calls GET /api/v1/analytics/top-filers.
func (c *APIClient) GetTopFilers(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/analytics/top-filers", params))
}

// GetFilingDistribution calls GET /api/v1/analytics/filing-distribution.
func (c *APIClient) GetFilingDistribution(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/analytics/filing-distribution", params))
}

// GetFilingTrends calls GET /api/v1/analytics/trends.
func (c *APIClient) GetFilingTrends(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/analytics/trends", params))
}

// GetLaunchHistory calls GET /api/v1/launches.
func (c *APIClient) GetLaunchHistory(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/launches", params))
}

// GetBondPortfolio calls GET /api/v1/bonds.
func (c *APIClient) GetBondPortfolio(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/bonds", params))
}

// GetBondSummary calls GET /api/v1/bonds/summary.
func (c *APIClient) GetBondSummary(ctx context.Context) (json.RawMessage, error) {
	return c.doGet(ctx, "/api/v1/bonds/summary")
}

// GetFinancialResponsibility calls GET /api/v1/financial-responsibility.
func (c *APIClient) GetFinancialResponsibility(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/financial-responsibility", params))
}

// GetMilestones calls GET /api/v1/milestones.
func (c *APIClient) GetMilestones(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	return c.doGet(ctx, buildPath("/api/v1/milestones", params))
}

// GetMilestonesSummary calls GET /api/v1/milestones/summary.
func (c *APIClient) GetMilestonesSummary(ctx context.Context) (json.RawMessage, error) {
	return c.doGet(ctx, "/api/v1/milestones/summary")
}

// Research leg names. These are contract: they appear verbatim in the
// machine-readable leg-status block that formatResearch emits.
const (
	legFilings  = "filings"
	legEntities = "entities"
	legSemantic = "semantic"
)

// Leg statuses.
const (
	legOK      = "ok"      // the leg ran and its results are included
	legFailed  = "failed"  // the leg errored; the others are still included
	legSkipped = "skipped" // the leg was not run
)

// ResearchLeg is the outcome of one of the parallel searches.
type ResearchLeg struct {
	Leg    string `json:"leg"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// ResearchResult holds the combined results of parallel API searches.
//
// A leg that fails must never discard the legs that succeeded, so every leg
// reports independently and the caller always gets whatever came back. Legs
// is written to fixed slots rather than appended, so its order does not
// depend on which goroutine finished first.
type ResearchResult struct {
	Filings  json.RawMessage
	Entities json.RawMessage
	Semantic json.RawMessage
	Legs     []ResearchLeg
}

// Failed reports whether any leg did not return results.
func (r *ResearchResult) Failed() []ResearchLeg {
	var out []ResearchLeg
	for _, l := range r.Legs {
		if l.Status != legOK {
			out = append(out, l)
		}
	}
	return out
}

// Research performs parallel filing search, entity search, and semantic search.
func (c *APIClient) Research(ctx context.Context, question string, focus string, agency string) (*ResearchResult, error) {
	var wg sync.WaitGroup
	res := &ResearchResult{Legs: make([]ResearchLeg, 3)}
	var mu sync.Mutex

	// Fixed slots keep the reported order stable regardless of finish order.
	res.Legs[0] = ResearchLeg{Leg: legFilings, Status: legOK}
	res.Legs[1] = ResearchLeg{Leg: legEntities, Status: legOK}
	res.Legs[2] = ResearchLeg{Leg: legSemantic, Status: legOK}

	filingKeywords := extractFilingKeywords(question)
	entityKeywords := extractEntityNames(question)

	// Filing search — uses full-text search on title/summary
	wg.Add(1)
	go func() {
		defer wg.Done()
		params := map[string]string{"per_page": "5"}
		if focus == "filing" || focus == "spectrum" {
			params["per_page"] = "10"
		}
		if agency != "" {
			params["agency"] = agency
		}
		if filingKeywords != "" {
			params["q"] = filingKeywords
		}
		data, err := c.SearchFilings(ctx, params)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			res.Legs[0] = ResearchLeg{Leg: legFilings, Status: legFailed, Detail: err.Error()}
		} else {
			res.Filings = data
		}
	}()

	// Entity search — uses fuzzy name matching
	wg.Add(1)
	go func() {
		defer wg.Done()
		params := map[string]string{"per_page": "5"}
		if focus == "entity" {
			params["per_page"] = "10"
		}
		if entityKeywords != "" {
			params["q"] = entityKeywords
		}
		data, err := c.SearchEntities(ctx, params)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			res.Legs[1] = ResearchLeg{Leg: legEntities, Status: legFailed, Detail: err.Error()}
		} else {
			res.Entities = data
		}
	}()

	// Semantic search — handles natural language natively
	wg.Add(1)
	go func() {
		defer wg.Done()
		limit := 5
		if focus == "filing" || focus == "spectrum" {
			limit = 10
		}
		data, err := c.SearchSemantic(ctx, question, limit, agency, 0)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			res.Legs[2] = ResearchLeg{Leg: legSemantic, Status: legFailed, Detail: err.Error()}
		} else {
			res.Semantic = data
		}
	}()

	wg.Wait()
	return res, nil
}

// extractFilingKeywords strips question/stop words and returns terms suitable
// for PostgreSQL plainto_tsquery. Returns empty string if no meaningful terms remain.
func extractFilingKeywords(question string) string {
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true, "was": true, "were": true,
		"what": true, "which": true, "who": true, "whom": true, "how": true, "when": true,
		"where": true, "why": true, "do": true, "does": true, "did": true, "can": true,
		"could": true, "would": true, "should": true, "will": true, "shall": true,
		"have": true, "has": true, "had": true, "be": true, "been": true, "being": true,
		"i": true, "me": true, "my": true, "we": true, "our": true, "you": true, "your": true,
		"it": true, "its": true, "this": true, "that": true, "these": true, "those": true,
		"of": true, "in": true, "on": true, "at": true, "to": true, "for": true, "with": true,
		"by": true, "from": true, "about": true, "between": true, "through": true, "into": true,
		"and": true, "or": true, "but": true, "not": true, "no": true, "nor": true,
		"most": true, "more": true, "less": true, "very": true, "just": true, "also": true,
		"than": true, "then": true, "so": true, "if": true, "as": true, "up": true,
		"all": true, "any": true, "each": true, "every": true, "both": true, "few": true,
		"some": true, "such": true, "other": true, "only": true,
		// Agency names — these are passed as filters, not text search
		"fcc": true, "itu": true, "unoosa": true, "un_oosa": true, "faa": true, "noaa": true,
		// Domain-generic terms that match too broadly
		"filing": true, "filings": true, "file": true, "filed": true, "filer": true, "filers": true,
		"entity": true, "entities": true, "organization": true, "organizations": true,
		"company": true, "companies": true, "past": true, "recent": true, "last": true, "recently": true,
		"year": true, "years": true, "month": true, "months": true, "day": true, "days": true,
		"year-over-year": true, "yoy": true, "prior": true, "vs": true, "versus": true,
		"show": true, "find": true, "list": true, "get": true, "give": true, "tell": true,
		"notable": true, "unusual": true, "interesting": true, "important": true, "biggest": true,
		"largest": true, "top": true, "active": true, "count": true, "number": true, "counts": true,
		"breakdown": true, "comparison": true, "trend": true, "trends": true, "increase": true,
		"decrease": true, "growth": true, "change": true, "rate": true, "volume": true,
		"related": true, "subsidiaries": true, "parents": true, "partners": true,
	}

	words := strings.Fields(strings.ToLower(question))
	var kept []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}*-")
		if w == "" || len(w) <= 1 {
			continue
		}
		if _, err := strconv.Atoi(w); err == nil {
			continue
		}
		if !stopWords[w] {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

// containsWord checks if text contains term as a whole word (not substring).
func containsWord(text, term string) bool {
	idx := 0
	for {
		pos := strings.Index(text[idx:], term)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		// Check word boundary before
		if abs > 0 {
			prev := text[abs-1]
			if prev != ' ' && prev != ',' && prev != ';' && prev != '.' && prev != '-' && prev != '/' && prev != '&' {
				idx = abs + len(term)
				continue
			}
		}
		// Check word boundary after
		end := abs + len(term)
		if end < len(text) {
			next := text[end]
			if next != ' ' && next != ',' && next != ';' && next != '.' && next != '-' && next != '/' && next != '&' && next != '\'' {
				idx = abs + len(term)
				continue
			}
		}
		return true
	}
}

// extractEntityNames pulls potential entity/organization names from a question.
// Returns empty string if no recognizable names are found, which causes the
// entity search to return all entities (useful for broad analytical queries).
func extractEntityNames(question string) string {
	knownTerms := []string{
		"spacex", "starlink", "intelsat", "viasat", "kuiper", "amazon",
		"oneweb", "worldvu", "telesat", "iridium", "globalstar", "orbcomm",
		"hughes", "echostar", "dish", "directv", "boeing", "lockheed", "northrop",
		"spire", "planet", "maxar", "l3harris", "thales", "airbus", "arianespace",
		"lynk", "astranis", "kepler", "leosat", "rivada", "mangata",
		"t-mobile", "verizon", "comcast",
	}
	// Short terms that need word-boundary matching to avoid false positives
	shortTerms := []string{"ses", "ast", "at&t"}

	lower := strings.ToLower(question)
	var found []string
	for _, term := range knownTerms {
		if strings.Contains(lower, term) {
			found = append(found, term)
		}
	}
	for _, term := range shortTerms {
		if containsWord(lower, term) {
			found = append(found, term)
		}
	}

	if len(found) > 0 {
		return strings.Join(found, " ")
	}

	// Fall back to the question's own keywords and let the API pick the terms.
	//
	// This used to harvest any 2-6 character all-caps word as an acronym,
	// which is wrong in a way that only shows up on real traffic: a client
	// writing "NORAD OR COSPAR OR ARKTIKA-M" has every "OR" harvested, and
	// hyphenated designators are too long to survive the length test. One
	// client's entity searches were literally
	//
	//	NORAD OR OR COSPAR OR OR OR OR OR N2 OR № OR N2 OR OR NORAD OR ...
	//
	// for months — 519 requests, every one HTTP 200, every one zero results.
	//
	// Picking terms out of a question needs to know how common each one is in
	// the entity corpus, which only the server knows, so the right split is
	// for this to send the substantive words and for GET /api/v1/entities to
	// weight them. extractFilingKeywords already does exactly the stripping
	// wanted here — question words, agency names, generic filing vocabulary —
	// so the entity and filing legs now search on the same cleaned text.
	return extractFilingKeywords(question)
}

func buildPath(base string, params map[string]string) string {
	if len(params) == 0 {
		return base
	}
	v := url.Values{}
	for k, val := range params {
		if val != "" {
			v.Set(k, val)
		}
	}
	if encoded := v.Encode(); encoded != "" {
		return base + "?" + encoded
	}
	return base
}

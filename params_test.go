package main

import (
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

// testdata/api_route_params.json is the API's query-key allowlist per GET route
// (routeParams in orbit-sentinel cmd/api/catalog.go); the API answers any other
// key with 400 unknown_param. Taken from orbit-sentinel 30a5dee (catalog.go as
// of b045ec1). Refresh it after the API's allowlist changes, from the deployed
// API (its /api/v1/meta lists the same keys; routes it does not check show []):
//
//	curl -s https://orbit-sentinel.viventine.com/api/v1/meta | jq -S \
//	  '[.endpoints[].items[] | select(.method=="GET") | {key: .path, value: (.params // [])}] | from_entries' \
//	  > testdata/api_route_params.json
//
// TestToolQueryKeysAllowedByAPI calls every tool with every argument set and
// fails if a tool sends a key its route does not accept: such a call used to be
// a silent wrong 200 and is now a 400 (BUGLIST day-3 #11).
func loadRouteParams(t *testing.T) map[string][]string {
	t.Helper()
	b, err := os.ReadFile("testdata/api_route_params.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string][]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// matchRoute returns the route pattern (with {id} segments) that path matches.
func matchRoute(routes map[string][]string, path string) (string, bool) {
	segs := strings.Split(path, "/")
	for pattern := range routes {
		ps := strings.Split(pattern, "/")
		if len(ps) != len(segs) {
			continue
		}
		ok := true
		for i := range ps {
			if ps[i] != segs[i] && !(strings.HasPrefix(ps[i], "{") && segs[i] != "") {
				ok = false
				break
			}
		}
		if ok {
			return pattern, true
		}
	}
	return "", false
}

type sentRequest struct {
	method, path string
	keys         []string
}

// argsFor sets every property of a tool's input schema: strings "x", numbers
// 1, booleans b.
func argsFor(t *testing.T, schema any, b bool) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(schema)
	var s struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{}
	for name, p := range s.Properties {
		types, _ := json.Marshal(p.Type)
		switch {
		case strings.Contains(string(types), "boolean"):
			args[name] = b
		case strings.Contains(string(types), "integer"), strings.Contains(string(types), "number"):
			args[name] = 1
		default:
			args[name] = "x"
		}
	}
	return args
}

func TestToolQueryKeysAllowedByAPI(t *testing.T) {
	routes := loadRouteParams(t)
	var mu sync.Mutex
	sent := map[string][]sentRequest{}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var keys []string
		for k := range r.URL.Query() {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		mu.Lock()
		tool := r.Header.Get("X-MCP-Tool")
		sent[tool] = append(sent[tool], sentRequest{r.Method, r.URL.Path, keys})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	cs := newTestSession(t, api)
	tools, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		// Both boolean settings: some flags switch the endpoint
		// (milestone_adherence summary=true calls /milestones/summary).
		for _, b := range []bool{true, false} {
			callText(t, cs, tool.Name, argsFor(t, tool.InputSchema, b))
		}
		if len(sent[tool.Name]) == 0 {
			t.Errorf("%s made no API request", tool.Name)
		}
		for _, req := range sent[tool.Name] {
			if req.method != http.MethodGet || !strings.HasPrefix(req.path, "/api/v1/") {
				continue // POST bodies and /status are not allowlisted
			}
			route, ok := matchRoute(routes, req.path)
			if !ok {
				t.Errorf("%s: %s is not an API route in the snapshot", tool.Name, req.path)
				continue
			}
			for _, k := range req.keys {
				if !slices.Contains(routes[route], k) {
					t.Errorf("%s: %s sends %q, which the API rejects (accepted: %v)", tool.Name, route, k, routes[route])
				}
			}
		}
	}
}

// Every key the API accepts on the SSAL and ground-station routes can be sent,
// and a false boolean filter is sent as false rather than dropped (#44).
func TestToolsSendNewFilters(t *testing.T) {
	routes := loadRouteParams(t)
	var mu sync.Mutex
	got := map[string]map[string]string{}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got[r.URL.Path] = map[string]string{}
		for k := range r.URL.Query() {
			got[r.URL.Path][k] = r.URL.Query().Get(k)
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"total":0}`))
	})
	cs := newTestSession(t, api)

	callText(t, cs, "search_space_station_authorizations", map[string]any{
		"operator": "telesat", "system": "lightspeed", "licensing_admin": "Canada", "call_sign": "S1234",
		"entity_id": "e", "is_ngso": true, "foreign": true, "granted": false, "count_only": true, "limit": 10,
	})
	ssal := got["/api/v1/space-station-authorizations"]
	for _, k := range routes["/api/v1/space-station-authorizations"] {
		if _, ok := ssal[k]; !ok {
			t.Errorf("search_space_station_authorizations cannot send %q (sent %v)", k, ssal)
		}
	}
	if ssal["granted"] != "false" || ssal["is_ngso"] != "true" {
		t.Errorf("boolean filters: %v", ssal)
	}
	callText(t, cs, "search_space_station_authorizations", map[string]any{"operator": "telesat"})
	for _, k := range []string{"is_ngso", "foreign", "granted", "count_only"} {
		if _, ok := got["/api/v1/space-station-authorizations"][k]; ok {
			t.Errorf("unset %s was sent", k)
		}
	}

	callText(t, cs, "search_ground_stations", map[string]any{"state": "AK", "group_by": "operator"})
	if gs := got["/api/v1/ground-stations"]; gs["state"] != "AK" || gs["group_by"] != "operator" {
		t.Errorf("ground stations: %v", gs)
	}
}

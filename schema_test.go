package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The enums orbit-sentinel's api/handlers/query.go accepts (validAgencies,
// validFilingTypes, validStatuses). search_filings used to list 5 agencies,
// "etc." for types and describe status as orbital (BUGLIST day-3 #20).
var (
	apiAgencies = []string{"ACMA_AU", "CNES_FR", "FAA_AST", "FCC", "ITU", "NOAA", "NTIA", "OFCOM_UK", "OTHER",
		"SEC", "STATE_AK", "STATE_FL", "STATE_TX", "STATE_VA", "UN_OOSA", "USPTO"}
	apiFilingTypes = []string{"ADVANCE_PUBLICATION", "COMMENT", "EARTH_STATION_PERMIT", "ENVIRONMENTAL_ASSESSMENT",
		"EXPERIMENTAL_LICENSE", "EX_PARTE", "LAUNCH_LICENSE", "MODIFICATION", "OPPOSITION", "ORBITAL_DEBRIS_PLAN",
		"OTHER", "PETITION", "REENTRY_LICENSE", "REGISTRATION", "REMOTE_SENSING_LICENSE", "RENEWAL",
		"SATELLITE_LICENSE", "SPECIAL_TEMPORARY_AUTH", "SPECTRUM_COORDINATION", "TRANSFER_OF_CONTROL"}
	apiStatuses = []string{"ACCEPTED", "CANCELLED", "COMMENT_PERIOD", "DENIED", "DISMISSED", "EXPIRED", "FILED",
		"GRANTED", "PUBLIC_NOTICE", "UNDER_REVIEW", "UNKNOWN", "WITHDRAWN"}
)

// toolProps returns each tool's input property descriptions.
func toolProps(t *testing.T) map[string]map[string]string {
	t.Helper()
	cs := newTestSession(t, jsonAPI(map[string]any{}))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, tool := range res.Tools {
		b, _ := json.Marshal(tool.InputSchema)
		var s struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		out[tool.Name] = map[string]string{}
		for k, p := range s.Properties {
			out[tool.Name][k] = p.Description
		}
	}
	return out
}

func TestSearchFilingsSchemaListsAPIValues(t *testing.T) {
	props := toolProps(t)
	sf := props["search_filings"]
	for prop, want := range map[string][]string{"agency": apiAgencies, "type": apiFilingTypes, "status": apiStatuses} {
		for _, v := range want {
			if !strings.Contains(sf[prop], v) {
				t.Errorf("search_filings.%s does not list %s: %q", prop, v, sf[prop])
			}
		}
	}
	if strings.Contains(sf["status"], "orbit") {
		t.Errorf("search_filings.status describes orbital status: %q", sf["status"])
	}
	for _, p := range []string{"source_id", "source_id_prefix", "call_sign", "entity_id"} {
		if sf[p] == "" {
			t.Errorf("search_filings has no %s parameter", p)
		}
	}
	for _, v := range apiAgencies {
		if !strings.Contains(props["search_semantic"]["agency"], v) {
			t.Errorf("search_semantic.agency does not list %s", v)
		}
	}
	if !strings.Contains(props["get_filing_detail"]["id"], "file number") {
		t.Errorf("get_filing_detail.id does not mention file numbers: %q", props["get_filing_detail"]["id"])
	}
}

// Schema text follows how the API matches (BUGLIST day-3 #36, #37, #44).
func TestSchemaTextMatchesAPIBehavior(t *testing.T) {
	props := toolProps(t)
	se := props["search_entities"]
	for _, v := range []string{"company", "individual", "government", "corporation"} {
		if !strings.Contains(se["type"], v) {
			t.Errorf("search_entities.type does not list %s: %q", v, se["type"])
		}
	}
	if strings.Contains(se["type"], "OPERATOR") || strings.Contains(se["type"], "MANUFACTURER") {
		t.Errorf("search_entities.type lists values the API does not store: %q", se["type"])
	}
	if !strings.Contains(se["country"], "name") {
		t.Errorf("search_entities.country does not say it accepts a country name: %q", se["country"])
	}
	if q := props["search_positions"]["q"]; !strings.Contains(q, "every word") || strings.Contains(q, "Substring") {
		t.Errorf("search_positions.q does not describe all-words matching: %q", q)
	}
	gs := props["search_ground_stations"]
	if gs["state"] == "" || !strings.Contains(gs["group_by"], "state") || !strings.Contains(gs["group_by"], "operator") {
		t.Errorf("search_ground_stations lacks state/group_by: %q / %q", gs["state"], gs["group_by"])
	}
	if strings.Contains(gs["band"], "requires source=extracted") || !strings.Contains(gs["band"], "entirely within") {
		t.Errorf("search_ground_stations.band misdescribes band matching: %q", gs["band"])
	}
	for _, src := range []string{"acma", "ised", "ofcom"} {
		if !strings.Contains(gs["source"], src) {
			t.Errorf("search_ground_stations.source does not list %s: %q", src, gs["source"])
		}
	}
}

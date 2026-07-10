package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Tool input types ---

type searchFilingsInput struct {
	Query       string `json:"q,omitempty" jsonschema:"Search query for filing titles and summaries"`
	Agency      string `json:"agency,omitempty" jsonschema:"Filter by source agency (FCC, ITU, UN_OOSA, FAA_AST, NOAA)"`
	Type        string `json:"type,omitempty" jsonschema:"Filter by filing type (SATELLITE_LICENSE, EARTH_STATION_PERMIT, SPECTRUM_COORDINATION, etc.)"`
	Status      string `json:"status,omitempty" jsonschema:"Filter by status (FILED, GRANTED, DENIED, UNDER_REVIEW, etc.)"`
	Docket      string `json:"docket,omitempty" jsonschema:"Filter by docket number (exact match)"`
	FiledAfter  string `json:"filed_after,omitempty" jsonschema:"Minimum filed date (YYYY-MM-DD)"`
	FiledBefore string `json:"filed_before,omitempty" jsonschema:"Maximum filed date (YYYY-MM-DD)"`
	Page        int    `json:"page,omitempty" jsonschema:"Page number (default 1)"`
	PerPage     int    `json:"per_page,omitempty" jsonschema:"Results per page (default 10, max 100)"`
	CountOnly   bool   `json:"count_only,omitempty" jsonschema:"If true, return only {total: N} without fetching rows"`
}

type getFilingInput struct {
	ID string `json:"id" jsonschema:"Filing UUID"`
}

type searchSemanticInput struct {
	Query         string  `json:"query" jsonschema:"Natural language search query"`
	Limit         int     `json:"limit,omitempty" jsonschema:"Max results (default 10, max 50)"`
	Agency        string  `json:"agency,omitempty" jsonschema:"Filter by source agency"`
	MinSimilarity float64 `json:"min_similarity,omitempty" jsonschema:"Minimum cosine similarity threshold (default 0.5)"`
	FiledAfter    string  `json:"filed_after,omitempty" jsonschema:"Minimum filed date (YYYY-MM-DD)"`
	FiledBefore   string  `json:"filed_before,omitempty" jsonschema:"Maximum filed date (YYYY-MM-DD)"`
}

type searchPositionsInput struct {
	Docket       string `json:"docket,omitempty" jsonschema:"Filter by docket number (e.g. '25-306')"`
	Stance       string `json:"stance,omitempty" jsonschema:"Filter by overall stance: support|oppose|qualified_support|qualified_opposition|informational"`
	ArgumentType string `json:"argument_type,omitempty" jsonschema:"Filter by argument type: legal|technical|economic|policy|procedural"`
	Position     string `json:"position,omitempty" jsonschema:"Filter by per-argument position: support|oppose|modify|neutral"`
	TargetParty  string `json:"target_party,omitempty" jsonschema:"Substring match on target_party (the entity being addressed or opposed)"`
	Filer        string `json:"filer,omitempty" jsonschema:"Substring match on the filing party's canonical name"`
	Query        string `json:"q,omitempty" jsonschema:"Substring search across argument text and executive summaries"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max results (default 25, max 200)"`
}

type searchEntitiesInput struct {
	Query   string `json:"q,omitempty" jsonschema:"Fuzzy search on entity name and aliases"`
	Type    string `json:"type,omitempty" jsonschema:"Filter by entity type (OPERATOR, MANUFACTURER, GOVERNMENT, etc.)"`
	Country string `json:"country,omitempty" jsonschema:"Filter by country code (US, GB, FR, etc.)"`
	Page    int    `json:"page,omitempty" jsonschema:"Page number (default 1)"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"Results per page (default 25, max 100)"`
}

type getEntityProfileInput struct {
	ID          string `json:"id" jsonschema:"Entity UUID"`
	FiledAfter  string `json:"filed_after,omitempty" jsonschema:"Filter filing stats to filings after this date (YYYY-MM-DD)"`
	FiledBefore string `json:"filed_before,omitempty" jsonschema:"Filter filing stats to filings before this date (YYYY-MM-DD)"`
}

type searchSpectrumInput struct {
	FreqLowMHz   string `json:"freq_low_mhz,omitempty" jsonschema:"Lower bound of the band to search, in MHz"`
	FreqHighMHz  string `json:"freq_high_mhz,omitempty" jsonschema:"Upper bound of the band to search, in MHz"`
	Agency       string `json:"agency,omitempty" jsonschema:"Filter by source agency (FCC, ITU, ...)"`
	Direction    string `json:"direction,omitempty" jsonschema:"Filter by direction (e.g. uplink, downlink)"`
	Polarization string `json:"polarization,omitempty" jsonschema:"Filter by polarization"`
	Holder       string `json:"holder,omitempty" jsonschema:"Substring match on the allocation holder (applicant entity)"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 500)"`
}

type searchSECInput struct {
	Ticker   string `json:"ticker,omitempty" jsonschema:"Company ticker (e.g. ASTS)"`
	CIK      string `json:"cik,omitempty" jsonschema:"SEC CIK number"`
	Company  string `json:"company,omitempty" jsonschema:"Substring match on company name"`
	EntityID string `json:"entity_id,omitempty" jsonschema:"Filter by resolved entity UUID"`
	FormType string `json:"form_type,omitempty" jsonschema:"SEC form type (e.g. 8-K, 10-Q, 10-K)"`
	Since    string `json:"since,omitempty" jsonschema:"Only filings on/after this date (YYYY-MM-DD)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 500)"`
}

type searchScreeningInput struct {
	EntityID      string `json:"entity_id,omitempty" jsonschema:"Filter by resolved entity UUID"`
	Name          string `json:"name,omitempty" jsonschema:"Substring match on entity or matched name"`
	List          string `json:"list,omitempty" jsonschema:"Substring match on screening list source (e.g. SDN, Entity List, ITAR)"`
	MinSimilarity string `json:"min_similarity,omitempty" jsonschema:"Minimum match similarity 0-1 (e.g. 0.9)"`
	Limit         int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 500)"`
}

type getDossierInput struct {
	ID               string `json:"id" jsonschema:"Entity UUID"`
	IncludeFamily      bool   `json:"include_family,omitempty" jsonschema:"Roll the dossier up across the entity's corporate family (same legal entity grouped by shared CIK/FRN or matching name). Default false."`
	FamilyConfidence   string `json:"family_confidence,omitempty" jsonschema:"Family grouping strictness when include_family=true: 'high' (default; exact name / shared identifier) or 'medium' (also groups normalized-name matches like 'AT&T INC.' with 'AT&T Corp.')."`
	IncludeSubsidiaries bool  `json:"include_subsidiaries,omitempty" jsonschema:"Also roll up the entity's direct subsidiaries (from authoritative SEC Exhibit-21 / GCAT parent links). Default false."`
}

type searchSatellitesInput struct {
	Name       string `json:"name,omitempty" jsonschema:"Substring match on satellite name"`
	Operator   string `json:"operator,omitempty" jsonschema:"Substring match on operator name (e.g. SpaceX); note many rows use a country code (US, CIS, PRC) as the operator"`
	Country    string `json:"country,omitempty" jsonschema:"Operator country code (exact match)"`
	OrbitClass string `json:"orbit_class,omitempty" jsonschema:"Orbit class (e.g. LEO, MEO, GEO)"`
	Status     string `json:"status,omitempty" jsonschema:"Orbital status (e.g. active, decayed)"`
	COSPAR     string `json:"cospar,omitempty" jsonschema:"COSPAR / international designator (exact match)"`
	NORAD      string `json:"norad,omitempty" jsonschema:"NORAD catalog id (integer)"`
	EntityID   string `json:"entity_id,omitempty" jsonschema:"Filter by resolved operator entity UUID"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 500)"`
}

type searchFederalAwardsInput struct {
	Recipient string `json:"recipient,omitempty" jsonschema:"Substring match on recipient/company name (e.g. boeing)"`
	Agency    string `json:"agency,omitempty" jsonschema:"Substring match on awarding/sub/funding agency (e.g. NASA, Department of Defense)"`
	AwardType string `json:"award_type,omitempty" jsonschema:"Award type: 'contract' or 'idv'"`
	NAICS     string `json:"naics,omitempty" jsonschema:"NAICS code prefix (e.g. 5415 for computer systems design)"`
	EntityID  string `json:"entity_id,omitempty" jsonschema:"Filter by resolved recipient entity UUID"`
	MinAmount string `json:"min_amount,omitempty" jsonschema:"Minimum award amount in USD (e.g. 1000000000 for $1B+)"`
	Since     string `json:"since,omitempty" jsonschema:"Only awards starting on/after this date (YYYY-MM-DD)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 500)"`
}

type searchGroundStationsInput struct {
	Near     string `json:"near,omitempty" jsonschema:"Proximity anchor as 'lat,lon' (e.g. '38.9,-77.0'); returns stations within radius_km ordered by distance"`
	RadiusKM string `json:"radius_km,omitempty" jsonschema:"Search radius in km for near= (default 500)"`
	Name     string `json:"name,omitempty" jsonschema:"Substring match on station name / FCC call sign"`
	Band     string `json:"band,omitempty" jsonschema:"Frequency band, exact match against the station's band list (e.g. Ku); requires source=extracted"`
	Operator string `json:"operator,omitempty" jsonschema:"Substring match on operator / licensee name"`
	EntityID string `json:"entity_id,omitempty" jsonschema:"Filter by resolved operator entity UUID; requires source=extracted"`
	Source   string `json:"source,omitempty" jsonschema:"Dataset: 'fcc' (authoritative FCC IBFS registry, reliable coordinates) or 'extracted' (entity-linked, LLM-extracted from filings, carries bands). Defaults to fcc for proximity, extracted for band/entity searches."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 500)"`
}

type researchInput struct {
	Question string `json:"question" jsonschema:"Natural language question about space regulatory filings, entities, or spectrum"`
	Focus    string `json:"focus,omitempty" jsonschema:"Optional focus area: filing, entity, spectrum, or general (default: general)"`
	Agency   string `json:"agency,omitempty" jsonschema:"Filter all sub-queries by agency (FCC, ITU, UN_OOSA). Recommended for agency-specific questions."`
}

type getTopFilersInput struct {
	Agency      string `json:"agency,omitempty" jsonschema:"Filter by agency (FCC, ITU, UN_OOSA)"`
	EntityType  string `json:"entity_type,omitempty" jsonschema:"Filter by entity type: company, individual, or government"`
	FiledAfter  string `json:"filed_after,omitempty" jsonschema:"Start date (YYYY-MM-DD)"`
	FiledBefore string `json:"filed_before,omitempty" jsonschema:"End date (YYYY-MM-DD)"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Max entities (default 10, max 100)"`
}

type getFilingDistributionInput struct {
	Agency      string `json:"agency,omitempty" jsonschema:"Filter by agency (FCC, ITU, UN_OOSA)"`
	EntityType  string `json:"entity_type,omitempty" jsonschema:"Filter by entity type: company, individual, or government"`
	FiledAfter  string `json:"filed_after,omitempty" jsonschema:"Start date (YYYY-MM-DD)"`
	FiledBefore string `json:"filed_before,omitempty" jsonschema:"End date (YYYY-MM-DD)"`
}

type getFilingTrendsInput struct {
	Agency       string `json:"agency,omitempty" jsonschema:"Filter by agency (FCC, ITU, UN_OOSA)"`
	EntityType   string `json:"entity_type,omitempty" jsonschema:"Filter by entity type: company, individual, or government"`
	EntityID     string `json:"entity_id,omitempty" jsonschema:"Entity UUID for per-entity trends"`
	Periods      int    `json:"periods,omitempty" jsonschema:"Number of periods (default 2, max 12)"`
	PeriodMonths int    `json:"period_months,omitempty" jsonschema:"Months per period (default 12, max 60)"`
	TopMovers    int    `json:"top_movers,omitempty" jsonschema:"Top N entities by biggest change"`
}

type getLaunchHistoryInput struct {
	EntityID string `json:"entity_id" jsonschema:"Entity UUID to look up launch history for"`
	Vehicle  string `json:"vehicle,omitempty" jsonschema:"Filter by vehicle type (e.g., Falcon 9, Electron)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max launches to return (default 20, max 100)"`
}

type getBondPortfolioInput struct {
	OrbitType string `json:"orbit_type,omitempty" jsonschema:"Filter by orbit type: NGSO or GSO"`
	Operator  string `json:"operator,omitempty" jsonschema:"Search operator name (partial match)"`
	EntityID  string `json:"entity_id,omitempty" jsonschema:"Filter by entity UUID"`
	Page      int    `json:"page,omitempty" jsonschema:"Page number (default 1)"`
	PerPage   int    `json:"per_page,omitempty" jsonschema:"Results per page (default 20, max 100)"`
}

type milestoneAdherenceInput struct {
	CallSign       string `json:"call_sign,omitempty" jsonschema:"Filter by FCC call sign (exact match)"`
	Classification string `json:"classification,omitempty" jsonschema:"Filter by classification: met|met_late|pending|extended|waived|missed|missed_unverified|unknown"`
	IsNGSO         *bool  `json:"is_ngso,omitempty" jsonschema:"Filter by orbit type: true for NGSO, false for GSO"`
	Summary        bool   `json:"summary,omitempty" jsonschema:"If true, return aggregate counts by classification/orbit type instead of individual rows"`
}

// --- Response types for JSON parsing ---

type paginationData struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type listEnvelope struct {
	Data       json.RawMessage `json:"data"`
	Pagination paginationData  `json:"pagination"`
}

type filingListItem struct {
	ID            string  `json:"id"`
	SourceAgency  string  `json:"source_agency"`
	SourceID      string  `json:"source_id"`
	FilingType    string  `json:"filing_type"`
	Title         *string `json:"title"`
	Status        string  `json:"status"`
	FiledDate     *string `json:"filed_date"`
	ApplicantName *string `json:"applicant_name"`
}

type entitySummary struct {
	ID            string  `json:"id"`
	CanonicalName string  `json:"canonical_name"`
	EntityType    *string `json:"entity_type"`
	Country       *string `json:"country"`
}

type spectrumItem struct {
	BandDesignation *string  `json:"band_designation"`
	FrequencyLow    *float64 `json:"frequency_low_mhz"`
	FrequencyHigh   *float64 `json:"frequency_high_mhz"`
	Direction       *string  `json:"direction"`
	EIRP            *float64 `json:"eirp_dbw"`
	Polarization    *string  `json:"polarization"`
	Confidence      *string  `json:"confidence"`
}

type orbitalItem struct {
	OrbitType         *string  `json:"orbit_type"`
	AltitudeKm        *float64 `json:"altitude_km"`
	InclinationDeg    *float64 `json:"inclination_deg"`
	Eccentricity      *float64 `json:"eccentricity"`
	NumSatsPlanned    *int     `json:"num_satellites_planned"`
	ConstellationName *string  `json:"constellation_name"`
	OrbitalPlane      *string  `json:"orbital_plane"`
	Confidence        *string  `json:"confidence"`
}

type groundStationItem struct {
	StationName     *string  `json:"station_name"`
	Latitude        *float64 `json:"latitude"`
	Longitude       *float64 `json:"longitude"`
	AntennaDiameter *float64 `json:"antenna_diameter_m"`
	Confidence      *string  `json:"confidence"`
}

type signalItem struct {
	SignalType  string   `json:"signal_type"`
	Description *string  `json:"description"`
	Severity    *string  `json:"severity"`
	Confidence  *float64 `json:"confidence"`
	DetectedAt  *string  `json:"detected_at"`
}

type attachmentItem struct {
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	FileSize    int     `json:"file_size_bytes"`
	PageCount   *int    `json:"page_count"`
	Stored      bool    `json:"stored"`
	DownloadURL *string `json:"download_url"`
}

type eventItem struct {
	EventType   string  `json:"event_type"`
	EventDate   *string `json:"event_date"`
	OldStatus   *string `json:"old_status"`
	NewStatus   *string `json:"new_status"`
	Description *string `json:"description"`
}

type relatedFilingItem struct {
	ID               string  `json:"id"`
	SourceID         string  `json:"source_id"`
	Title            *string `json:"title"`
	RelationshipType string  `json:"relationship_type"`
	Direction        string  `json:"direction"`
}

type filingDetail struct {
	ID               string              `json:"id"`
	SourceAgency     string              `json:"source_agency"`
	SourceID         string              `json:"source_id"`
	FilingType       string              `json:"filing_type"`
	Title            *string             `json:"title"`
	Summary          *string             `json:"summary"`
	Status           string              `json:"status"`
	FiledDate        *string             `json:"filed_date"`
	EffectiveDate    *string             `json:"effective_date"`
	ExpirationDate   *string             `json:"expiration_date"`
	DocketNumber     *string             `json:"docket_number"`
	CallSign         *string             `json:"call_sign"`
	SourceURL        *string             `json:"source_url"`
	ExtractionStatus string              `json:"extraction_status"`
	Applicant        *entitySummary      `json:"applicant"`
	Attachments      []attachmentItem    `json:"attachments"`
	Events           []eventItem         `json:"events"`
	RelatedFilings   []relatedFilingItem `json:"related_filings"`
	SpectrumData     []spectrumItem      `json:"spectrum_data"`
	OrbitalParams    []orbitalItem       `json:"orbital_parameters"`
	GroundStations   []groundStationItem `json:"ground_stations"`
	Signals          []signalItem        `json:"signals"`
	LongForm         *longFormItem       `json:"long_form,omitempty"`
	Position         *positionItem       `json:"position,omitempty"`
	Arguments        []argumentItem      `json:"arguments,omitempty"`
}

type longFormItem struct {
	ExecutiveSummary string `json:"executive_summary"`
	WordCount        *int   `json:"word_count"`
	ReadingTimeMin   *int   `json:"reading_time_min"`
}

type positionItem struct {
	OverallStance         *string  `json:"overall_stance"`
	Tone                  *string  `json:"tone"`
	PrimaryRecommendation *string  `json:"primary_recommendation"`
	RuleCitations         []string `json:"rule_citations"`
	Confidence            *float64 `json:"confidence"`
}

type argumentItem struct {
	ArgumentType *string  `json:"argument_type"`
	Position     *string  `json:"position"`
	ArgumentText string   `json:"argument_text"`
	Target       *string  `json:"target"`
	TargetParty  *string  `json:"target_party"`
	SourceQuote  *string  `json:"source_quote"`
	SourcePage   *int     `json:"source_page"`
	Confidence   *float64 `json:"confidence"`
}

type entityListItem struct {
	ID            string   `json:"id"`
	CanonicalName string   `json:"canonical_name"`
	Aliases       []string `json:"aliases"`
	EntityType    *string  `json:"entity_type"`
	Country       *string  `json:"country"`
	FCCFRN        *string  `json:"fcc_frn"`
	CoresFRN      *string  `json:"cores_frn"`
	FilingCount   int      `json:"filing_count"`
}

type relatedEntityItem struct {
	ID            string  `json:"id"`
	CanonicalName string  `json:"canonical_name"`
	EntityType    *string `json:"entity_type"`
	Country       *string `json:"country"`
	SharedDockets int     `json:"shared_dockets"`
}

type satelliteItem struct {
	Name          string  `json:"name"`
	NORADCatID    *int    `json:"norad_cat_id"`
	COSPARID      *string `json:"cospar_id"`
	OrbitClass    *string `json:"orbit_class"`
	OrbitalStatus *string `json:"orbital_status"`
}

type entityLinkItem struct {
	EntityID      string  `json:"entity_id"`
	CanonicalName string  `json:"canonical_name"`
	LinkType      string  `json:"link_type"`
	Confidence    *string `json:"confidence"`
}

type entityProfile struct {
	ID               string               `json:"id"`
	CanonicalName    string               `json:"canonical_name"`
	Aliases          []string             `json:"aliases"`
	EntityType       *string              `json:"entity_type"`
	Country          *string              `json:"country"`
	SECCIK           *string              `json:"sec_cik"`
	FCCFRN           *string              `json:"fcc_frn"`
	CoresFRN         *string              `json:"cores_frn"`
	Website          *string              `json:"website"`
	FilingCount      int                  `json:"filing_count"`
	FilingStats      map[string]int       `json:"filing_stats"`
	EarliestFiling   *string              `json:"earliest_filing"`
	LatestFiling     *string              `json:"latest_filing"`
	RelatedEntities  []relatedEntityItem  `json:"related_entities"`
	Satellites       []satelliteItem      `json:"satellites"`
	EntityLinks      []entityLinkItem     `json:"entity_links"`
	Dockets          []docketItem         `json:"dockets"`
	InsuranceRisk    *insuranceRiskData   `json:"insurance_risk,omitempty"`
	IndustryData     []marketDataItem     `json:"industry_data,omitempty"`
	ScreeningMatches []screeningMatchItem `json:"screening_matches,omitempty"`
}

type screeningMatchItem struct {
	ScreeningID string   `json:"screening_id"`
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	MatchType   string   `json:"match_type"`
	Similarity  float32  `json:"similarity"`
	MatchedName string   `json:"matched_name"`
	Country     string   `json:"country,omitempty"`
	Programs    []string `json:"programs,omitempty"`
}

type insuranceRiskData struct {
	FAAMPL       *faaMPLData     `json:"faa_mpl,omitempty"`
	FCCBonds     *fccBondsData   `json:"fcc_bonds,omitempty"`
	LossHistory  []lossEventData `json:"loss_history,omitempty"`
	AnomalyCount int             `json:"anomaly_count"`
}

type faaMPLData struct {
	Licenses       []faaLicenseData `json:"licenses"`
	TotalLiability int64            `json:"total_liability_usd"`
}

type faaLicenseData struct {
	LicenseNumber  string  `json:"license_number"`
	Operator       string  `json:"operator"`
	VehicleType    string  `json:"vehicle_type"`
	LicenseType    string  `json:"license_type"`
	PreflightTPL   *int64  `json:"preflight_tpl_usd,omitempty"`
	FlightTPL      *int64  `json:"flight_tpl_usd,omitempty"`
	GovtProperty   *int64  `json:"govt_property_usd,omitempty"`
	ReentryTPL     *int64  `json:"reentry_tpl_usd,omitempty"`
	EffectiveDate  *string `json:"effective_date,omitempty"`
	ExpirationDate *string `json:"expiration_date,omitempty"`
}

type fccBondsData struct {
	ActiveBonds    int   `json:"active_bonds"`
	TotalBondValue int64 `json:"total_bond_value_usd"`
}

type lossEventData struct {
	Source    string `json:"source"`
	Type      string `json:"type"`
	Year      int    `json:"year"`
	Operator  string `json:"operator"`
	Vehicle   string `json:"vehicle"`
	Mission   string `json:"mission"`
	AmountUSD *int64 `json:"amount_usd,omitempty"`
}

type marketDataItem struct {
	Source     string   `json:"source"`
	Domain     string   `json:"domain"`
	RecordType string   `json:"record_type"`
	Year       int      `json:"year"`
	Operator   string   `json:"operator,omitempty"`
	Vehicle    string   `json:"vehicle,omitempty"`
	AmountUSD  *int64   `json:"amount_usd,omitempty"`
	Ratio      *float64 `json:"ratio,omitempty"`
	Count      *int     `json:"count,omitempty"`
}

type semanticSearchResponse struct {
	Results             []semanticResult `json:"results"`
	QueryEmbeddingModel string           `json:"query_embedding_model"`
	TotalChunksSearched int              `json:"total_chunks_searched"`
}

type semanticResult struct {
	FilingID      string  `json:"filing_id"`
	SourceID      string  `json:"source_id"`
	Title         *string `json:"title"`
	Summary       *string `json:"summary"`
	Similarity    float64 `json:"similarity"`
	MatchedChunk  string  `json:"matched_chunk"`
	Agency        string  `json:"agency"`
	FiledDate     *string `json:"filed_date"`
	ApplicantName *string `json:"applicant_name"`
}

type statusResponse struct {
	Status   string `json:"status"`
	Database struct {
		Connected         bool   `json:"connected"`
		ActiveConnections int    `json:"active_connections"`
		TotalConnections  int    `json:"total_connections"`
		DatabaseSize      string `json:"database_size"`
	} `json:"database"`
	Pipeline struct {
		Pending    int `json:"pending"`
		Processing int `json:"processing"`
		Completed  int `json:"completed"`
		Failed     int `json:"failed"`
	} `json:"pipeline"`
	Sources []struct {
		Agency    string `json:"agency"`
		LastCrawl string `json:"last_crawl"`
		DocsFound int    `json:"documents_found"`
		DocsNew   int    `json:"documents_new"`
	} `json:"sources"`
}

type topFilersResponse struct {
	Filers     []topFilerItem   `json:"filers"`
	TotalCount int              `json:"total_count"`
	Filters    analyticsFilters `json:"filters"`
}

type topFilerItem struct {
	Rank          int    `json:"rank"`
	EntityID      string `json:"entity_id"`
	CanonicalName string `json:"canonical_name"`
	FilingCount   int    `json:"filing_count"`
}

type filingDistributionResponse struct {
	Distribution []filingTypeCount `json:"distribution"`
	TotalCount   int               `json:"total_count"`
	Filters      analyticsFilters  `json:"filters"`
}

type filingTypeCount struct {
	FilingType string  `json:"filing_type"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type trendsResponse struct {
	Periods   []trendPeriod `json:"periods"`
	TopMovers []trendMover  `json:"top_movers"`
	Filters   trendFilters  `json:"filters"`
}

type trendPeriod struct {
	PeriodStart string   `json:"period_start"`
	PeriodEnd   string   `json:"period_end"`
	FilingCount int      `json:"filing_count"`
	Delta       *int     `json:"delta"`
	PctChange   *float64 `json:"pct_change"`
}

type trendMover struct {
	EntityID      string   `json:"entity_id"`
	CanonicalName string   `json:"canonical_name"`
	CurrentCount  int      `json:"current_count"`
	PreviousCount int      `json:"previous_count"`
	Delta         int      `json:"delta"`
	PctChange     *float64 `json:"pct_change"`
}

type trendFilters struct {
	Agency       string `json:"agency"`
	EntityID     string `json:"entity_id"`
	Periods      int    `json:"periods"`
	PeriodMonths int    `json:"period_months"`
}

type analyticsFilters struct {
	Agency      string `json:"agency"`
	FiledAfter  string `json:"filed_after"`
	FiledBefore string `json:"filed_before"`
}

type docketItem struct {
	DocketNumber string `json:"docket_number"`
	FilingCount  int    `json:"filing_count"`
}

// --- Tool registration ---

// wrapAddTool wraps mcp.AddTool to stamp the tool name onto every request
// context. Downstream REST calls in cmd/mcp/client.go read the name back
// off the context and set X-MCP-Tool, which the API audit middleware records.
//
// Renamed from `addTool` after a runtime stack overflow at startup — when
// the helper shared a case-folded name with the SDK's `AddTool`, generic
// instantiation resolved the inner call to the helper itself instead of
// the SDK, causing infinite recursion. Distinct name avoids that whatever
// the root cause is.
func wrapAddTool[In, Out any](s *mcp.Server, tool *mcp.Tool, handler func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error)) {
	name := tool.Name
	mcp.AddTool(s, tool, func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		return handler(WithMCPTool(ctx, name), req, input)
	})
}

func registerTools(s *mcp.Server, client *APIClient) {
	wrapAddTool(s, &mcp.Tool{
		Name:        "research",
		Description: "Primary research tool — searches filings, entities, and semantic index in parallel. Use this FIRST for any question. Pass the agency parameter for agency-specific questions (e.g., agency=\"FCC\" for FCC questions).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input researchInput) (*mcp.CallToolResult, any, error) {
		if input.Question == "" {
			return textResult("Error: question is required"), nil, nil
		}
		result, err := client.Research(ctx, input.Question, input.Focus, input.Agency)
		if err != nil {
			return textResult("Error: " + err.Error()), nil, nil
		}
		return textResult(formatResearch(result)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_filings",
		Description: "Search space regulatory filings by keyword, agency, type, status, and date range. Returns a paginated list of filings from FCC, ITU, UNOOSA, and other agencies.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchFilingsInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"q": input.Query, "agency": input.Agency, "type": input.Type,
			"status": input.Status, "docket": input.Docket,
			"filed_after": input.FiledAfter, "filed_before": input.FiledBefore,
		}
		if input.CountOnly {
			params["count_only"] = "true"
		}
		if input.Page > 0 {
			params["page"] = strconv.Itoa(input.Page)
		}
		perPage := input.PerPage
		if perPage <= 0 {
			perPage = 10
		}
		params["per_page"] = strconv.Itoa(perPage)
		data, err := client.SearchFilings(ctx, params)
		if err != nil {
			return textResult("Error searching filings: " + err.Error()), nil, nil
		}
		if input.CountOnly {
			var countResp struct {
				Total int `json:"total"`
			}
			if err := json.Unmarshal(data, &countResp); err != nil {
				return textResult("Error parsing count response: " + err.Error()), nil, nil
			}
			return textResult(fmt.Sprintf("**Total filings matching query:** %d", countResp.Total)), nil, nil
		}
		return textResult(formatFilingList(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "get_filing_detail",
		Description: "Get full details of a specific regulatory filing including spectrum data, orbital parameters, ground stations, signals, related filings, attachments, and (for policy filings) executive summary, overall stance, and structured arguments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getFilingInput) (*mcp.CallToolResult, any, error) {
		data, err := client.GetFiling(ctx, input.ID)
		if err != nil {
			return textResult("Error fetching filing: " + err.Error()), nil, nil
		}
		return textResult(formatFiling(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_positions",
		Description: "Search LLM-extracted policy arguments across COMMENT / REPLY / PETITION filings. Filter by docket, overall stance, argument type, target party, or filer. Use this to answer questions like 'who opposed X?', 'what did SpaceX argue in 25-306?', 'which filings support modular satellite licensing?'. Returns one row per (filing, argument).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchPositionsInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"docket":        input.Docket,
			"stance":        input.Stance,
			"argument_type": input.ArgumentType,
			"position":      input.Position,
			"target_party":  input.TargetParty,
			"filer":         input.Filer,
			"q":             input.Query,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.SearchPositions(ctx, params)
		if err != nil {
			return textResult("Error searching positions: " + err.Error()), nil, nil
		}
		return textResult(formatPositionSearch(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_semantic",
		Description: "Semantic vector search across filing text using natural language. Uses nomic-embed-text-v1.5 embeddings to find filings by meaning, not just keywords. Note: FCC filings have limited embedding coverage — use search_filings for FCC keyword search.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchSemanticInput) (*mcp.CallToolResult, any, error) {
		data, err := client.SearchSemanticFull(ctx, SemanticSearchParams(input))
		if err != nil {
			return textResult("Error in semantic search: " + err.Error()), nil, nil
		}
		return textResult(formatSemanticResults(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_entities",
		Description: "Search regulatory entities (companies, operators, manufacturers, governments) by name, type, or country. Supports fuzzy name matching.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchEntitiesInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"q": input.Query, "type": input.Type, "country": input.Country,
		}
		if input.Page > 0 {
			params["page"] = strconv.Itoa(input.Page)
		}
		perPage := input.PerPage
		if perPage <= 0 {
			perPage = 10
		}
		params["per_page"] = strconv.Itoa(perPage)
		data, err := client.SearchEntities(ctx, params)
		if err != nil {
			return textResult("Error searching entities: " + err.Error()), nil, nil
		}
		return textResult(formatEntityList(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "get_entity_profile",
		Description: "Get a detailed entity profile including filing history by agency, related entities (co-filers), linked satellites, and cross-references.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getEntityProfileInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{}
		if input.FiledAfter != "" {
			params["filed_after"] = input.FiledAfter
		}
		if input.FiledBefore != "" {
			params["filed_before"] = input.FiledBefore
		}
		data, err := client.GetEntityProfile(ctx, input.ID, params)
		if err != nil {
			return textResult("Error fetching entity: " + err.Error()), nil, nil
		}
		return textResult(formatEntity(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "get_system_status",
		Description: "Get system health status including database connectivity, pipeline queue depth, and per-source crawl health.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		data, err := client.GetStatus(ctx)
		if err != nil {
			return textResult("Error fetching status: " + err.Error()), nil, nil
		}
		return textResult(formatStatus(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "get_top_filers",
		Description: "Get the top filing entities ranked by number of filings. Filter by agency and date range. Use for questions about who files the most, biggest players, or filing rankings.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getTopFilersInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"agency": input.Agency, "entity_type": input.EntityType,
			"filed_after": input.FiledAfter, "filed_before": input.FiledBefore,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.GetTopFilers(ctx, params)
		if err != nil {
			return textResult("Error fetching top filers: " + err.Error()), nil, nil
		}
		return textResult(formatTopFilers(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "get_filing_distribution",
		Description: "Get filing count distribution by filing type. Filter by agency and date range. Use for questions about what types of filings are most common.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getFilingDistributionInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"agency": input.Agency, "entity_type": input.EntityType,
			"filed_after": input.FiledAfter, "filed_before": input.FiledBefore,
		}
		data, err := client.GetFilingDistribution(ctx, params)
		if err != nil {
			return textResult("Error fetching distribution: " + err.Error()), nil, nil
		}
		return textResult(formatFilingDistribution(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "get_filing_trends",
		Description: "Get filing volume trends over time periods with optional top movers (entities with biggest changes). Use for questions about growth, trends, emerging players, or year-over-year comparisons.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getFilingTrendsInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{"agency": input.Agency, "entity_type": input.EntityType, "entity_id": input.EntityID}
		if input.Periods > 0 {
			params["periods"] = strconv.Itoa(input.Periods)
		}
		if input.PeriodMonths > 0 {
			params["period_months"] = strconv.Itoa(input.PeriodMonths)
		}
		if input.TopMovers > 0 {
			params["top_movers"] = strconv.Itoa(input.TopMovers)
		}
		data, err := client.GetFilingTrends(ctx, params)
		if err != nil {
			return textResult("Error fetching trends: " + err.Error()), nil, nil
		}
		return textResult(formatTrends(data)), nil, nil
	})

	// Launch history tool — queries faa_launch_operations via API
	wrapAddTool(s, &mcp.Tool{
		Name:        "get_launch_history",
		Description: "Get launch history for a space entity. Returns launches from the FAA/GCAT database including vehicle type, launch site, outcome, and date. Requires entity_id.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getLaunchHistoryInput) (*mcp.CallToolResult, any, error) {
		if input.EntityID == "" {
			return textResult("Error: entity_id is required"), nil, nil
		}
		params := map[string]string{"entity_id": input.EntityID}
		if input.Vehicle != "" {
			params["vehicle"] = input.Vehicle
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.GetLaunchHistory(ctx, params)
		if err != nil {
			return textResult("Error fetching launch history: " + err.Error()), nil, nil
		}
		return textResult(formatLaunchHistory(data)), nil, nil
	})

	// Bond portfolio tool — FCC surety bonds + FAA financial responsibility
	wrapAddTool(s, &mcp.Tool{
		Name:        "get_bond_portfolio",
		Description: "Get FCC surety bond portfolio for satellite operators per 47 CFR 25.165. Shows active/theoretical bond amounts, bond releases, and operator details. Also returns summary statistics and FAA financial responsibility data. Use for questions about satellite operator financial obligations, bond compliance, or TPL coverage.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getBondPortfolioInput) (*mcp.CallToolResult, any, error) {
		// Always fetch summary
		summary, summaryErr := client.GetBondSummary(ctx)

		// Fetch portfolio with filters
		params := map[string]string{}
		if input.OrbitType != "" {
			params["orbit_type"] = input.OrbitType
		}
		if input.Operator != "" {
			params["operator"] = input.Operator
		}
		if input.EntityID != "" {
			params["entity_id"] = input.EntityID
		}
		if input.Page > 0 {
			params["page"] = strconv.Itoa(input.Page)
		}
		if input.PerPage > 0 {
			params["per_page"] = strconv.Itoa(input.PerPage)
		}
		portfolio, portfolioErr := client.GetBondPortfolio(ctx, params)

		// Also fetch FAA financial responsibility if entity_id specified
		var faaData json.RawMessage
		if input.EntityID != "" {
			faaData, _ = client.GetFinancialResponsibility(ctx, map[string]string{"entity_id": input.EntityID})
		}

		return textResult(formatBondPortfolio(summary, summaryErr, portfolio, portfolioErr, faaData)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_spectrum",
		Description: "Find which entities hold a frequency band. Given a range in MHz, returns spectrum allocations overlapping it, joined to the source filing and applicant — e.g. 'who is allocated 11700-12200 MHz downlink?'. Filter by agency, direction, polarization, or holder name.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchSpectrumInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"freq_low_mhz": input.FreqLowMHz, "freq_high_mhz": input.FreqHighMHz,
			"agency": input.Agency, "direction": input.Direction,
			"polarization": input.Polarization, "holder": input.Holder,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.SearchSpectrum(ctx, params)
		if err != nil {
			return textResult("Error searching spectrum: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_sec_filings",
		Description: "Search SEC filings for tracked space companies (8-K material events, 10-Q, 10-K). Filter by ticker, CIK, company name, resolved entity, form type, or date. Use for financial signals — e.g. recent 8-Ks for a satellite operator.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchSECInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"ticker": input.Ticker, "cik": input.CIK, "company": input.Company,
			"entity_id": input.EntityID, "form_type": input.FormType, "since": input.Since,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.SearchSECFilings(ctx, params)
		if err != nil {
			return textResult("Error searching SEC filings: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_screening",
		Description: "Check entities against consolidated sanctions / export-control screening lists (OFAC SDN, BIS Entity List, ITAR Debarred, etc.). Filter by entity, name, list source, or minimum match similarity. Use for compliance / due-diligence questions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchScreeningInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"entity_id": input.EntityID, "name": input.Name,
			"list": input.List, "min_similarity": input.MinSimilarity,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.SearchScreening(ctx, params)
		if err != nil {
			return textResult("Error searching screening lists: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "get_entity_dossier",
		Description: "Get a cross-source dossier for an entity (by UUID): regulatory filings, SEC financial signals, sanctions/export-control screening hits, and asset footprint (satellites, ground stations, federal awards, surety bonds) — counts plus recent samples in one call. The most complete single view of an operator. Set include_family=true to roll the totals up across the entity's corporate family.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getDossierInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{}
		if input.IncludeFamily {
			params["include_family"] = "true"
		}
		if input.FamilyConfidence != "" {
			params["family_confidence"] = input.FamilyConfidence
		}
		if input.IncludeSubsidiaries {
			params["include_subsidiaries"] = "true"
		}
		data, err := client.GetEntityDossier(ctx, input.ID, params)
		if err != nil {
			return textResult("Error fetching dossier: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_satellites",
		Description: "Search the satellite catalog (UCS + Space-Track SATCAT) by name, operator, country, orbit class, status, COSPAR, or NORAD id. Joined to the resolved operator entity. Use for 'what does SpaceX have in LEO?', 'find NORAD 44713', or building an operator's fleet.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchSatellitesInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"name": input.Name, "operator": input.Operator, "country": input.Country,
			"orbit_class": input.OrbitClass, "status": input.Status,
			"cospar": input.COSPAR, "norad": input.NORAD, "entity_id": input.EntityID,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.SearchSatellites(ctx, params)
		if err != nil {
			return textResult("Error searching satellites: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_ground_stations",
		Description: "Search ground / earth stations by name, frequency band, operator, or geographic proximity (near='lat,lon' within radius_km, ordered by great-circle distance). Proximity defaults to the authoritative FCC IBFS registry (reliable coordinates); band/entity searches use the entity-linked extracted set. Each result is labeled with its source. Use for 'earth stations within 200km of 38.9,-77.0' or an operator's gateway footprint.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchGroundStationsInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"near": input.Near, "radius_km": input.RadiusKM, "name": input.Name,
			"band": input.Band, "operator": input.Operator, "entity_id": input.EntityID,
			"source": input.Source,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.SearchGroundStations(ctx, params)
		if err != nil {
			return textResult("Error searching ground stations: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "search_federal_awards",
		Description: "Search U.S. federal awards (USAspending contracts + IDVs) by recipient, agency, award type, NAICS, or minimum amount, joined to the resolved recipient entity. Ordered by award amount (largest first). Use for 'NASA contracts to Boeing over $1B' or an operator's federal funding footprint.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchFederalAwardsInput) (*mcp.CallToolResult, any, error) {
		params := map[string]string{
			"recipient": input.Recipient, "agency": input.Agency, "award_type": input.AwardType,
			"naics": input.NAICS, "entity_id": input.EntityID,
			"min_amount": input.MinAmount, "since": input.Since,
		}
		if input.Limit > 0 {
			params["limit"] = strconv.Itoa(input.Limit)
		}
		data, err := client.SearchFederalAwards(ctx, params)
		if err != nil {
			return textResult("Error searching federal awards: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})

	wrapAddTool(s, &mcp.Tool{
		Name:        "milestone_adherence",
		Description: "FCC deployment milestone adherence (47 CFR 25.164): which authorized satellite systems met their deployment milestones. Filter by call_sign, classification (met|pending|extended|waived|missed|missed_unverified|unknown), is_ngso; set summary=true for aggregate counts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input milestoneAdherenceInput) (*mcp.CallToolResult, any, error) {
		if input.Summary {
			data, err := client.GetMilestonesSummary(ctx)
			if err != nil {
				return textResult("Error fetching milestone summary: " + err.Error()), nil, nil
			}
			return textResult(formatJSON(data)), nil, nil
		}
		params := map[string]string{"call_sign": input.CallSign, "classification": input.Classification}
		if input.IsNGSO != nil {
			params["is_ngso"] = strconv.FormatBool(*input.IsNGSO)
		}
		data, err := client.GetMilestones(ctx, params)
		if err != nil {
			return textResult("Error fetching milestones: " + err.Error()), nil, nil
		}
		return textResult(formatJSON(data)), nil, nil
	})
}

// formatJSON pretty-prints an API JSON response for the model to consume.
func formatJSON(data json.RawMessage) string {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(b)
}

// --- Formatting functions ---

func formatFilingList(data json.RawMessage) string {
	var env listEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "Error parsing response: " + err.Error()
	}
	var items []filingListItem
	if err := json.Unmarshal(env.Data, &items); err != nil {
		return "Error parsing filings: " + err.Error()
	}

	var b strings.Builder
	p := env.Pagination
	if p.Total > 0 {
		fmt.Fprintf(&b, "Found %d filings (page %d of %d)\n\n", p.Total, p.Page, p.TotalPages)
	} else {
		fmt.Fprintf(&b, "Filings (page %d)\n\n", p.Page)
	}

	if len(items) == 0 {
		b.WriteString("No filings match the search criteria.\n")
		return b.String()
	}

	b.WriteString("| # | ID | Agency | Type | Title | Filed | Applicant |\n")
	b.WriteString("|---|----|--------|------|-------|-------|-----------|\n")
	for i, f := range items {
		num := (p.Page-1)*p.PerPage + i + 1
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | %s |\n",
			num, f.ID, f.SourceAgency, f.FilingType, truncate(deref(f.Title), 60),
			deref(f.FiledDate), deref(f.ApplicantName))
	}
	b.WriteString("\n_Source: Orbit Sentinel database. Only cite facts shown above._\n")
	return b.String()
}

func formatFiling(data json.RawMessage) string {
	var f filingDetail
	if err := json.Unmarshal(data, &f); err != nil {
		return "Error parsing filing: " + err.Error()
	}

	var b strings.Builder

	title := f.SourceID
	if f.Title != nil {
		title = *f.Title
	}
	fmt.Fprintf(&b, "# Filing: %s\n\n", title)
	fmt.Fprintf(&b, "**Agency:** %s | **Type:** %s | **Status:** %s\n", f.SourceAgency, f.FilingType, f.Status)
	fmt.Fprintf(&b, "**Source ID:** %s | **Filed:** %s\n", f.SourceID, deref(f.FiledDate))

	if f.EffectiveDate != nil || f.ExpirationDate != nil {
		fmt.Fprintf(&b, "**Effective:** %s | **Expires:** %s\n", deref(f.EffectiveDate), deref(f.ExpirationDate))
	}
	if f.DocketNumber != nil || f.CallSign != nil {
		fmt.Fprintf(&b, "**Docket:** %s | **Call Sign:** %s\n", deref(f.DocketNumber), deref(f.CallSign))
	}
	if f.SourceURL != nil {
		fmt.Fprintf(&b, "**Source:** %s\n", *f.SourceURL)
	}
	fmt.Fprintf(&b, "**Extraction:** %s\n", f.ExtractionStatus)
	if f.ExtractionStatus != "completed" {
		fmt.Fprintf(&b, "**Data Quality:** Extraction status is %s — extracted fields may be incomplete or absent.\n", f.ExtractionStatus)
	}

	if f.Applicant != nil {
		fmt.Fprintf(&b, "\n## Applicant\n%s (%s, %s)\n",
			f.Applicant.CanonicalName, deref(f.Applicant.EntityType), deref(f.Applicant.Country))
	}

	if f.Summary != nil && *f.Summary != "" {
		fmt.Fprintf(&b, "\n## Summary\n%s\n", *f.Summary)
	}

	if f.LongForm != nil && f.LongForm.ExecutiveSummary != "" {
		b.WriteString("\n## Executive Summary\n")
		b.WriteString(f.LongForm.ExecutiveSummary)
		b.WriteString("\n")
		if f.LongForm.WordCount != nil && f.LongForm.ReadingTimeMin != nil {
			fmt.Fprintf(&b, "\n_%d words, ~%d min read._\n", *f.LongForm.WordCount, *f.LongForm.ReadingTimeMin)
		}
	}

	if f.Position != nil {
		b.WriteString("\n## Position\n")
		if f.Position.OverallStance != nil {
			fmt.Fprintf(&b, "**Stance:** %s", *f.Position.OverallStance)
		}
		if f.Position.Tone != nil {
			fmt.Fprintf(&b, " | **Tone:** %s", *f.Position.Tone)
		}
		if f.Position.Confidence != nil {
			fmt.Fprintf(&b, " | **Confidence:** %.2f", *f.Position.Confidence)
		}
		b.WriteString("\n")
		if f.Position.PrimaryRecommendation != nil && *f.Position.PrimaryRecommendation != "" {
			fmt.Fprintf(&b, "\n**Primary recommendation:** %s\n", *f.Position.PrimaryRecommendation)
		}
		if len(f.Position.RuleCitations) > 0 {
			fmt.Fprintf(&b, "\n**Rule citations:** %s\n", strings.Join(f.Position.RuleCitations, "; "))
		}
	}

	if len(f.Arguments) > 0 {
		b.WriteString("\n## Arguments\n")
		b.WriteString("| Type | Position | Argument | Target party | Page | Conf |\n")
		b.WriteString("|------|----------|----------|--------------|------|------|\n")
		showArgs := f.Arguments
		if len(showArgs) > 20 {
			showArgs = showArgs[:20]
		}
		for _, a := range showArgs {
			conf := "-"
			if a.Confidence != nil {
				conf = strconv.FormatFloat(*a.Confidence, 'f', 2, 64)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
				deref(a.ArgumentType), deref(a.Position),
				truncate(a.ArgumentText, 100),
				deref(a.TargetParty),
				derefInt(a.SourcePage),
				conf,
			)
		}
		if len(f.Arguments) > 20 {
			fmt.Fprintf(&b, "\n_... and %d more arguments. Use `search_positions` to filter further._\n", len(f.Arguments)-20)
		}
		b.WriteString("\n_Arguments are LLM-extracted from the filing. Verify quoted text against the source document before citing._\n")
	}

	if len(f.SpectrumData) > 0 {
		b.WriteString("\n## Spectrum Data\n")
		b.WriteString("| Band | Low (MHz) | High (MHz) | Direction | EIRP (dBW) | Polarization | Confidence |\n")
		b.WriteString("|------|-----------|------------|-----------|------------|--------------|------------|\n")
		for _, s := range f.SpectrumData {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n",
				deref(s.BandDesignation), derefFloat(s.FrequencyLow, 1), derefFloat(s.FrequencyHigh, 1),
				deref(s.Direction), derefFloat(s.EIRP, 1), deref(s.Polarization), deref(s.Confidence))
		}
	}

	if len(f.OrbitalParams) > 0 {
		b.WriteString("\n## Orbital Parameters\n")
		b.WriteString("| Type | Altitude (km) | Incl. (deg) | Ecc. | Planned Sats | Constellation | Plane | Confidence |\n")
		b.WriteString("|------|---------------|-------------|------|--------------|---------------|-------|------------|\n")
		for _, o := range f.OrbitalParams {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
				deref(o.OrbitType), derefFloat(o.AltitudeKm, 0), derefFloat(o.InclinationDeg, 1),
				derefFloat(o.Eccentricity, 4), derefInt(o.NumSatsPlanned), deref(o.ConstellationName),
				deref(o.OrbitalPlane), deref(o.Confidence))
		}
	}

	if len(f.GroundStations) > 0 {
		b.WriteString("\n## Ground Stations\n")
		b.WriteString("| Name | Latitude | Longitude | Antenna (m) | Confidence |\n")
		b.WriteString("|------|----------|-----------|-------------|------------|\n")
		for _, g := range f.GroundStations {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				deref(g.StationName), derefFloat(g.Latitude, 4), derefFloat(g.Longitude, 4),
				derefFloat(g.AntennaDiameter, 1), deref(g.Confidence))
		}
	}

	// Check for low/medium confidence in extracted technical data
	hasLowConf := false
	for _, s := range f.SpectrumData {
		if s.Confidence != nil && (*s.Confidence == "LOW" || *s.Confidence == "MEDIUM") {
			hasLowConf = true
			break
		}
	}
	if !hasLowConf {
		for _, o := range f.OrbitalParams {
			if o.Confidence != nil && (*o.Confidence == "LOW" || *o.Confidence == "MEDIUM") {
				hasLowConf = true
				break
			}
		}
	}
	if !hasLowConf {
		for _, g := range f.GroundStations {
			if g.Confidence != nil && (*g.Confidence == "LOW" || *g.Confidence == "MEDIUM") {
				hasLowConf = true
				break
			}
		}
	}
	if hasLowConf {
		b.WriteString("\n*Note: Some rows above have LOW or MEDIUM confidence — verify against the source document before citing.*\n")
	}
	if f.SourceURL != nil && (len(f.SpectrumData) > 0 || len(f.OrbitalParams) > 0 || len(f.GroundStations) > 0) {
		fmt.Fprintf(&b, "\n**Source document:** %s — verify extracted parameters against this source.\n", *f.SourceURL)
	}

	if len(f.Signals) > 0 {
		b.WriteString("\n## Signals\n")
		b.WriteString("| Type | Severity | Confidence | Description |\n")
		b.WriteString("|------|----------|------------|-------------|\n")
		for _, s := range f.Signals {
			conf := "-"
			if s.Confidence != nil {
				conf = strconv.FormatFloat(*s.Confidence, 'f', 2, 64)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
				s.SignalType, deref(s.Severity), conf, truncate(deref(s.Description), 80))
		}
	}

	if len(f.RelatedFilings) > 0 {
		b.WriteString("\n## Related Filings\n")
		b.WriteString("| ID | Source ID | Relationship | Direction | Title |\n")
		b.WriteString("|----|-----------|--------------|-----------|-------|\n")
		showRelated := f.RelatedFilings
		if len(showRelated) > 10 {
			showRelated = showRelated[:10]
		}
		for _, r := range showRelated {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				r.ID, r.SourceID, r.RelationshipType, r.Direction, truncate(deref(r.Title), 60))
		}
		if len(f.RelatedFilings) > 10 {
			fmt.Fprintf(&b, "\n... and %d more related filings\n", len(f.RelatedFilings)-10)
		}
	}

	if len(f.Attachments) > 0 {
		b.WriteString("\n## Attachments\n")
		b.WriteString("| Filename | Type | Size | Pages | Stored |\n")
		b.WriteString("|----------|------|------|-------|--------|\n")
		showAttach := f.Attachments
		if len(showAttach) > 10 {
			showAttach = showAttach[:10]
		}
		for _, a := range showAttach {
			storedStr := "No"
			if a.Stored {
				storedStr = "Yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				truncate(a.Filename, 50), a.ContentType, fmtBytes(a.FileSize), derefInt(a.PageCount), storedStr)
		}
		if len(f.Attachments) > 10 {
			fmt.Fprintf(&b, "\n... and %d more attachments\n", len(f.Attachments)-10)
		}
	}

	if len(f.Events) > 0 {
		b.WriteString("\n## Timeline\n")
		b.WriteString("| Date | Event | Details |\n")
		b.WriteString("|------|-------|---------|\n")
		showEvents := f.Events
		if len(showEvents) > 10 {
			showEvents = showEvents[:10]
		}
		for _, e := range showEvents {
			detail := deref(e.Description)
			if e.OldStatus != nil && e.NewStatus != nil {
				detail = fmt.Sprintf("%s -> %s", *e.OldStatus, *e.NewStatus)
			} else if e.NewStatus != nil {
				detail = "-> " + *e.NewStatus
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", deref(e.EventDate), e.EventType, detail)
		}
		if len(f.Events) > 10 {
			fmt.Fprintf(&b, "\n... and %d more events\n", len(f.Events)-10)
		}
	}

	b.WriteString("\n---\n_Source: Orbit Sentinel database. Verify claims against the source document URL above when available._\n")

	return b.String()
}

func formatSemanticResults(data json.RawMessage) string {
	var resp semanticSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error parsing results: " + err.Error()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Semantic search results (%d matches, %d chunks searched, model: %s)\n\n",
		len(resp.Results), resp.TotalChunksSearched, resp.QueryEmbeddingModel)

	if len(resp.Results) == 0 {
		b.WriteString("No results found above the similarity threshold.\n")
		return b.String()
	}

	for i, r := range resp.Results {
		fmt.Fprintf(&b, "### %d. %s (similarity: %.3f)\n", i+1, truncate(deref(r.Title), 70), r.Similarity)
		fmt.Fprintf(&b, "**Agency:** %s | **Source ID:** %s | **Filed:** %s | **Applicant:** %s\n",
			r.Agency, r.SourceID, deref(r.FiledDate), deref(r.ApplicantName))
		fmt.Fprintf(&b, "**Filing ID:** %s\n", r.FilingID)
		chunk := r.MatchedChunk
		if len(chunk) > 200 {
			chunk = chunk[:197] + "..."
		}
		fmt.Fprintf(&b, "\n> %s\n\n", chunk)
	}
	b.WriteString("_Source: Orbit Sentinel database. Only cite facts shown above._\n")
	return b.String()
}

func formatEntityList(data json.RawMessage) string {
	var env listEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "Error parsing response: " + err.Error()
	}
	var items []entityListItem
	if err := json.Unmarshal(env.Data, &items); err != nil {
		return "Error parsing entities: " + err.Error()
	}

	var b strings.Builder
	p := env.Pagination
	if p.Total > 0 {
		fmt.Fprintf(&b, "Found %d entities (page %d of %d)\n\n", p.Total, p.Page, p.TotalPages)
	} else {
		fmt.Fprintf(&b, "Entities (page %d)\n\n", p.Page)
	}

	if len(items) == 0 {
		b.WriteString("No entities match the search criteria.\n")
		return b.String()
	}

	b.WriteString("| # | ID | Name | Type | Country | FRN | Filings |\n")
	b.WriteString("|---|----|------|------|---------|-----|---------|\n")
	for i, e := range items {
		num := (p.Page-1)*p.PerPage + i + 1
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | %d |\n",
			num, e.ID, e.CanonicalName, deref(e.EntityType), deref(e.Country), deref(e.FCCFRN), e.FilingCount)
	}
	b.WriteString("\n_Source: Orbit Sentinel database. Only cite facts shown above._\n")
	return b.String()
}

func formatEntity(data json.RawMessage) string {
	var e entityProfile
	if err := json.Unmarshal(data, &e); err != nil {
		return "Error parsing entity: " + err.Error()
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# Entity: %s\n\n", e.CanonicalName)
	fmt.Fprintf(&b, "**Type:** %s | **Country:** %s\n", deref(e.EntityType), deref(e.Country))

	ids := []string{}
	if e.CoresFRN != nil {
		ids = append(ids, "CORES FRN: "+*e.CoresFRN)
	}
	if e.FCCFRN != nil {
		ids = append(ids, "IBFS FRN: "+*e.FCCFRN)
	}
	if e.SECCIK != nil {
		ids = append(ids, "SEC CIK: "+*e.SECCIK)
	}
	if e.Website != nil {
		ids = append(ids, "Web: "+*e.Website)
	}
	if len(ids) > 0 {
		fmt.Fprintf(&b, "**Identifiers:** %s\n", strings.Join(ids, " | "))
	}

	fmt.Fprintf(&b, "**Total Filings:** %d", e.FilingCount)
	if e.EarliestFiling != nil && e.LatestFiling != nil {
		fmt.Fprintf(&b, " | **Active:** %s to %s", *e.EarliestFiling, *e.LatestFiling)
	}
	b.WriteString("\n")

	if len(e.FilingStats) > 0 {
		b.WriteString("\n## Filing Breakdown\n")
		b.WriteString("| Agency | Count |\n")
		b.WriteString("|--------|-------|\n")
		for agency, count := range e.FilingStats {
			fmt.Fprintf(&b, "| %s | %d |\n", agency, count)
		}
	}

	if len(e.Aliases) > 0 {
		b.WriteString("\n## Also Known As\n")
		for _, a := range e.Aliases {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}

	if len(e.RelatedEntities) > 0 {
		b.WriteString("\n## Related Entities\n")
		b.WriteString("| ID | Name | Type | Country | Shared Dockets |\n")
		b.WriteString("|----|------|------|---------|----------------|\n")
		showRelated := e.RelatedEntities
		if len(showRelated) > 10 {
			showRelated = showRelated[:10]
		}
		for _, r := range showRelated {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %d |\n",
				r.ID, r.CanonicalName, deref(r.EntityType), deref(r.Country), r.SharedDockets)
		}
	}

	if len(e.Satellites) > 0 {
		b.WriteString("\n## Satellites\n")
		b.WriteString("| Name | NORAD ID | COSPAR | Orbit | Status |\n")
		b.WriteString("|------|----------|--------|-------|--------|\n")
		for _, s := range e.Satellites {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				s.Name, derefInt(s.NORADCatID), deref(s.COSPARID), deref(s.OrbitClass), deref(s.OrbitalStatus))
		}
	}

	if len(e.EntityLinks) > 0 {
		b.WriteString("\n## Corporate Family & Relationships\n")
		b.WriteString("Relationships identified through entity resolution and manual corporate structure analysis.\n\n")
		b.WriteString("| Name | Link Type | Confidence |\n")
		b.WriteString("|------|-----------|------------|\n")
		for _, l := range e.EntityLinks {
			fmt.Fprintf(&b, "| %s | %s | %s |\n",
				l.CanonicalName, l.LinkType, deref(l.Confidence))
		}
	}

	if len(e.Dockets) > 0 {
		b.WriteString("\n## Dockets\n")
		b.WriteString("| Docket Number | Filings |\n")
		b.WriteString("|---------------|---------|\n")
		for _, d := range e.Dockets {
			fmt.Fprintf(&b, "| %s | %d |\n", d.DocketNumber, d.FilingCount)
		}
	}

	if e.InsuranceRisk != nil {
		ir := e.InsuranceRisk
		b.WriteString("\n## Insurance & Risk\n")
		if ir.FAAMPL != nil {
			fmt.Fprintf(&b, "**FAA Total Liability:** $%s\n", formatUSD(ir.FAAMPL.TotalLiability))
			b.WriteString("| License | Vehicle | Preflight TPL | Flight TPL | Reentry TPL | Govt Property | Effective | Expires |\n")
			b.WriteString("|---------|---------|---------------|------------|-------------|---------------|-----------|----------|\n")
			for _, l := range ir.FAAMPL.Licenses {
				fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
					l.LicenseNumber, l.VehicleType,
					formatUSDPtr(l.PreflightTPL), formatUSDPtr(l.FlightTPL), formatUSDPtr(l.ReentryTPL), formatUSDPtr(l.GovtProperty),
					deref(l.EffectiveDate), deref(l.ExpirationDate))
			}
		}
		if ir.FCCBonds != nil {
			fmt.Fprintf(&b, "\n**FCC Surety Bonds:** %d events, total $%s\n",
				ir.FCCBonds.ActiveBonds, formatUSD(ir.FCCBonds.TotalBondValue))
		}
		if ir.AnomalyCount > 0 {
			fmt.Fprintf(&b, "**Spacecraft Anomalies:** %d historical events\n", ir.AnomalyCount)
		}
		if len(ir.LossHistory) > 0 {
			b.WriteString("\n### Loss History\n")
			b.WriteString("| Year | Operator | Vehicle | Mission | Amount |\n")
			b.WriteString("|------|----------|---------|---------|--------|\n")
			for _, lh := range ir.LossHistory {
				fmt.Fprintf(&b, "| %d | %s | %s | %s | %s |\n",
					lh.Year, lh.Operator, lh.Vehicle, lh.Mission, formatUSDPtr(lh.AmountUSD))
			}
		}
	}

	if len(e.IndustryData) > 0 {
		b.WriteString("\n## Industry Data\n")
		b.WriteString("| Year | Domain | Type | Operator | Vehicle | Amount | Count |\n")
		b.WriteString("|------|--------|------|----------|---------|--------|-------|\n")
		for _, md := range e.IndustryData {
			op := md.Operator
			if op == "" {
				op = "-"
			}
			veh := md.Vehicle
			if veh == "" {
				veh = "-"
			}
			amt := "-"
			if md.AmountUSD != nil {
				amt = "$" + formatUSD(*md.AmountUSD)
			}
			cnt := "-"
			if md.Count != nil {
				cnt = fmt.Sprintf("%d", *md.Count)
			}
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | %s |\n",
				md.Year, md.Domain, md.RecordType, op, veh, amt, cnt)
		}
	}

	if len(e.ScreeningMatches) > 0 {
		b.WriteString("\n## Sanctions Screening Matches\n")
		b.WriteString("| Name | Source List | Match Type | Similarity | Country | Programs |\n")
		b.WriteString("|------|------------|------------|------------|---------|----------|\n")
		for _, m := range e.ScreeningMatches {
			progs := "-"
			if len(m.Programs) > 0 {
				progs = strings.Join(m.Programs, ", ")
			}
			country := m.Country
			if country == "" {
				country = "-"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %.2f | %s | %s |\n",
				m.Name, m.Source, m.MatchType, m.Similarity, country, progs)
		}
	}

	b.WriteString("\n---\n_Source: Orbit Sentinel database. Verify claims against the source document URL above when available._\n")

	return b.String()
}

func formatUSD(v int64) string {
	if v == 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", v)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatUSDPtr(v *int64) string {
	if v == nil {
		return "-"
	}
	return "$" + formatUSD(*v)
}

func formatLaunchHistory(data json.RawMessage) string {
	var env struct {
		Data  []json.RawMessage `json:"data"`
		Total int               `json:"total"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "Error parsing launch history: " + err.Error()
	}

	if len(env.Data) == 0 {
		return "No launch operations found for this entity."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Launch History (%d operations)\n\n", env.Total)
	b.WriteString("| Date | Vehicle | Site | Mission | Outcome |\n")
	b.WriteString("|------|---------|------|---------|----------|\n")

	for _, raw := range env.Data {
		var item struct {
			OperationDate string `json:"operation_date"`
			VehicleType   string `json:"vehicle_type"`
			LaunchSite    string `json:"launch_site"`
			MissionName   string `json:"mission_name"`
			Outcome       string `json:"outcome"`
			LaunchTag     string `json:"launch_tag"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			item.OperationDate, item.VehicleType, item.LaunchSite,
			item.MissionName, item.Outcome)
	}

	return b.String()
}

func formatStatus(data json.RawMessage) string {
	var s statusResponse
	if err := json.Unmarshal(data, &s); err != nil {
		return "Error parsing status: " + err.Error()
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# System Status: %s\n\n", strings.ToUpper(s.Status))

	db := s.Database
	connected := "No"
	if db.Connected {
		connected = "Yes"
	}
	b.WriteString("## Database\n")
	fmt.Fprintf(&b, "- Connected: %s\n", connected)
	fmt.Fprintf(&b, "- Connections: %d / %d\n", db.ActiveConnections, db.TotalConnections)
	fmt.Fprintf(&b, "- Size: %s\n", db.DatabaseSize)

	p := s.Pipeline
	b.WriteString("\n## Pipeline\n")
	b.WriteString("| Status | Count |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| Pending | %d |\n", p.Pending)
	fmt.Fprintf(&b, "| Processing | %d |\n", p.Processing)
	fmt.Fprintf(&b, "| Completed | %d |\n", p.Completed)
	fmt.Fprintf(&b, "| Failed | %d |\n", p.Failed)

	if len(s.Sources) > 0 {
		b.WriteString("\n## Source Health\n")
		b.WriteString("| Agency | Last Crawl | Documents | New |\n")
		b.WriteString("|--------|------------|-----------|-----|\n")
		for _, src := range s.Sources {
			fmt.Fprintf(&b, "| %s | %s | %d | %d |\n",
				src.Agency, src.LastCrawl, src.DocsFound, src.DocsNew)
		}
	}

	return b.String()
}

func formatResearch(res *ResearchResult) string {
	var b strings.Builder

	b.WriteString("# Research Results\n\n")

	if len(res.Errors) > 0 {
		b.WriteString("**Partial errors:** " + strings.Join(res.Errors, "; ") + "\n\n")
	}

	// Filings section
	if res.Filings != nil {
		b.WriteString("## Relevant Filings\n")
		b.WriteString(formatFilingList(res.Filings))
		b.WriteString("\n")
	}

	// Entities section
	if res.Entities != nil {
		b.WriteString("## Related Entities\n")
		b.WriteString(formatEntityList(res.Entities))
		b.WriteString("\n")
	}

	// Semantic section
	if res.Semantic != nil {
		b.WriteString("## Semantic Matches\n")
		b.WriteString(formatSemanticResults(res.Semantic))
	}

	b.WriteString("\n---\n")
	b.WriteString("**DATA PROVENANCE:** All results above come from the Orbit Sentinel database.\n")
	b.WriteString("Sources indexed: FCC ECFS (~16K public comments), FCC IBFS (~127K satellite/earth station licenses), ITU SNL (~2.7K satellite networks), UNOOSA (~1.8K registrations).\n")
	b.WriteString("NOT indexed: FCC ELS (experimental licenses), FCC ULS (terrestrial licenses), NOAA CRSRA, FAA launch licenses, CORES FRN registry.\n")
	b.WriteString("IMPORTANT: Only state facts shown above. Do not infer, speculate, or supplement with outside knowledge. If no results appear for a topic, say \"no matching data found in database\" — the information may exist in sources not yet indexed.\n")

	return b.String()
}

func formatTopFilers(data json.RawMessage) string {
	var resp topFilersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error parsing top filers: " + err.Error()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Top Filers (%d unique filers)\n\n", resp.TotalCount)

	if resp.Filters.Agency != "" {
		fmt.Fprintf(&b, "**Agency:** %s", resp.Filters.Agency)
		if resp.Filters.FiledAfter != "" || resp.Filters.FiledBefore != "" {
			fmt.Fprintf(&b, " | **Period:** %s to %s", resp.Filters.FiledAfter, resp.Filters.FiledBefore)
		}
		b.WriteString("\n\n")
	}

	if len(resp.Filers) == 0 {
		b.WriteString("No filers found matching the criteria.\n")
		return b.String()
	}

	b.WriteString("| Rank | Entity | Filings |\n")
	b.WriteString("|------|--------|---------|\n")
	for _, f := range resp.Filers {
		fmt.Fprintf(&b, "| %d | %s | %d |\n", f.Rank, f.CanonicalName, f.FilingCount)
	}
	b.WriteString("\n_Source: Orbit Sentinel database._\n")
	return b.String()
}

func formatFilingDistribution(data json.RawMessage) string {
	var resp filingDistributionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error parsing distribution: " + err.Error()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Filing Distribution (%d total filings)\n\n", resp.TotalCount)

	if resp.Filters.Agency != "" {
		fmt.Fprintf(&b, "**Agency:** %s\n\n", resp.Filters.Agency)
	}

	if len(resp.Distribution) == 0 {
		b.WriteString("No filings found matching the criteria.\n")
		return b.String()
	}

	b.WriteString("| Type | Count | % |\n")
	b.WriteString("|------|-------|---|\n")
	for _, d := range resp.Distribution {
		fmt.Fprintf(&b, "| %s | %d | %.1f%% |\n", d.FilingType, d.Count, d.Percentage)
	}
	b.WriteString("\n_Source: Orbit Sentinel database._\n")
	return b.String()
}

func formatTrends(data json.RawMessage) string {
	var resp trendsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error parsing trends: " + err.Error()
	}

	var b strings.Builder
	b.WriteString("# Filing Trends\n\n")

	if resp.Filters.Agency != "" {
		fmt.Fprintf(&b, "**Agency:** %s", resp.Filters.Agency)
	}
	if resp.Filters.EntityID != "" {
		fmt.Fprintf(&b, " | **Entity:** %s", resp.Filters.EntityID)
	}
	fmt.Fprintf(&b, " | **%d periods of %d months**\n\n", resp.Filters.Periods, resp.Filters.PeriodMonths)

	b.WriteString("| Period | Filings | Delta | Change |\n")
	b.WriteString("|--------|---------|-------|--------|\n")
	for _, p := range resp.Periods {
		delta := "-"
		pctChange := "-"
		if p.Delta != nil {
			delta = fmt.Sprintf("%+d", *p.Delta)
		}
		if p.PctChange != nil {
			pctChange = fmt.Sprintf("%+.1f%%", *p.PctChange)
		}
		fmt.Fprintf(&b, "| %s to %s | %d | %s | %s |\n",
			p.PeriodStart, p.PeriodEnd, p.FilingCount, delta, pctChange)
	}

	if len(resp.TopMovers) > 0 {
		b.WriteString("\n## Top Movers\n")
		b.WriteString("| Entity | Current | Previous | Delta | Change |\n")
		b.WriteString("|--------|---------|----------|-------|--------|\n")
		for _, m := range resp.TopMovers {
			pctChange := "-"
			if m.PctChange != nil {
				pctChange = fmt.Sprintf("%+.1f%%", *m.PctChange)
			}
			fmt.Fprintf(&b, "| %s | %d | %d | %+d | %s |\n",
				m.CanonicalName, m.CurrentCount, m.PreviousCount, m.Delta, pctChange)
		}
	}

	b.WriteString("\n_Source: Orbit Sentinel database._\n")
	return b.String()
}

// --- Helpers ---

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func deref(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

func derefInt(n *int) string {
	if n == nil {
		return "-"
	}
	return strconv.Itoa(*n)
}

func derefFloat(f *float64, prec int) string {
	if f == nil {
		return "-"
	}
	return strconv.FormatFloat(*f, 'f', prec, 64)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatBondPortfolio(summary json.RawMessage, summaryErr error, portfolio json.RawMessage, portfolioErr error, faaData json.RawMessage) string {
	var b strings.Builder

	// Summary section
	if summaryErr == nil && summary != nil {
		var s struct {
			TotalOperators     int     `json:"total_operators"`
			ActiveBonds        int     `json:"active_bonds"`
			ReleasedBonds      int     `json:"released_bonds"`
			TotalActiveBondUSD float64 `json:"total_active_bond_usd"`
			NGSOActiveBondUSD  float64 `json:"ngso_active_bond_usd"`
			GSOActiveBondUSD   float64 `json:"gso_active_bond_usd"`
			TotalEvents        int     `json:"total_events"`
		}
		if err := json.Unmarshal(summary, &s); err == nil {
			b.WriteString("# FCC Surety Bond Portfolio Summary\n\n")
			fmt.Fprintf(&b, "- **Total Operators:** %d (%d active, %d released)\n", s.TotalOperators, s.ActiveBonds, s.ReleasedBonds)
			fmt.Fprintf(&b, "- **Total Active Bond Exposure:** $%.0f\n", s.TotalActiveBondUSD)
			fmt.Fprintf(&b, "- **NGSO:** $%.0f | **GSO:** $%.0f\n", s.NGSOActiveBondUSD, s.GSOActiveBondUSD)
			fmt.Fprintf(&b, "- **Bond Events:** %d\n\n", s.TotalEvents)
		}
	}

	// Portfolio table
	if portfolioErr == nil && portfolio != nil {
		var env struct {
			Data       json.RawMessage `json:"data"`
			Pagination paginationData  `json:"pagination"`
		}
		if err := json.Unmarshal(portfolio, &env); err == nil {
			var items []struct {
				OperatorName       string  `json:"operator_name"`
				SystemName         string  `json:"system_name"`
				CallSign           string  `json:"call_sign"`
				OrbitType          string  `json:"orbit_type"`
				ActiveBondUSD      float64 `json:"active_bond_usd"`
				TheoreticalBondUSD float64 `json:"theoretical_bond_usd"`
				BondReleased       bool    `json:"bond_released"`
				DaysSince          int     `json:"days_since_authorization"`
			}
			if err := json.Unmarshal(env.Data, &items); err == nil {
				p := env.Pagination
				fmt.Fprintf(&b, "## Bond Portfolio (page %d of %d, %d total)\n\n", p.Page, p.TotalPages, p.Total)
				if len(items) > 0 {
					b.WriteString("| Operator | System | Call Sign | Orbit | Active Bond | Status |\n")
					b.WriteString("|----------|--------|-----------|-------|-------------|--------|\n")
					for _, item := range items {
						status := "Active"
						if item.BondReleased {
							status = "Released"
						}
						fmt.Fprintf(&b, "| %s | %s | %s | %s | $%.0f | %s |\n",
							truncate(item.OperatorName, 30), truncate(item.SystemName, 20),
							item.CallSign, item.OrbitType, item.ActiveBondUSD, status)
					}
				} else {
					b.WriteString("No bond portfolio entries match the criteria.\n")
				}
				b.WriteString("\n")
			}
		}
	}

	// FAA Financial Responsibility (if entity-specific)
	if faaData != nil {
		var env struct {
			Data       json.RawMessage `json:"data"`
			Pagination paginationData  `json:"pagination"`
		}
		if err := json.Unmarshal(faaData, &env); err == nil {
			var items []struct {
				Operator        string `json:"operator"`
				LicenseNumber   string `json:"license_number"`
				VehicleType     string `json:"vehicle_type"`
				LaunchSite      string `json:"launch_site"`
				PreflightTPLUSD *int64 `json:"preflight_tpl_usd"`
				FlightTPLUSD    *int64 `json:"flight_tpl_usd"`
				LicenseType     string `json:"license_type"`
			}
			if err := json.Unmarshal(env.Data, &items); err == nil && len(items) > 0 {
				b.WriteString("## FAA Financial Responsibility (TPL Coverage)\n\n")
				b.WriteString("| Operator | License | Vehicle | Site | Preflight TPL | Flight TPL |\n")
				b.WriteString("|----------|---------|---------|------|--------------|------------|\n")
				for _, item := range items {
					pf, fl := "N/A", "N/A"
					if item.PreflightTPLUSD != nil {
						pf = fmt.Sprintf("$%d", *item.PreflightTPLUSD)
					}
					if item.FlightTPLUSD != nil {
						fl = fmt.Sprintf("$%d", *item.FlightTPLUSD)
					}
					fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
						truncate(item.Operator, 25), item.LicenseNumber, item.VehicleType,
						truncate(item.LaunchSite, 20), pf, fl)
				}
				b.WriteString("\n")
			}
		}
	}

	if b.Len() == 0 {
		return "No bond portfolio data available."
	}
	b.WriteString("_Source: Orbit Sentinel — FCC 47 CFR 25.165 bond calculations + FAA TPL data._\n")
	return b.String()
}

func fmtBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

type positionSearchRow struct {
	FilingID              string   `json:"filing_id"`
	SourceID              string   `json:"source_id"`
	Title                 *string  `json:"title"`
	FiledDate             *string  `json:"filed_date"`
	DocketNumber          *string  `json:"docket_number"`
	Applicant             *string  `json:"applicant"`
	OverallStance         *string  `json:"overall_stance"`
	Tone                  *string  `json:"tone"`
	PrimaryRecommendation *string  `json:"primary_recommendation"`
	ArgumentType          *string  `json:"argument_type"`
	Position              *string  `json:"position"`
	ArgumentText          string   `json:"argument_text"`
	Target                *string  `json:"target"`
	TargetParty           *string  `json:"target_party"`
	SourcePage            *int     `json:"source_page"`
	Confidence            *float64 `json:"confidence"`
}

func formatPositionSearch(data json.RawMessage) string {
	var resp struct {
		Results []positionSearchRow `json:"results"`
		Total   int                 `json:"total"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error parsing position search response: " + err.Error()
	}

	var b strings.Builder
	if len(resp.Results) == 0 {
		return "No policy arguments match the search criteria.\n"
	}

	fmt.Fprintf(&b, "Position search results (%d arguments)\n\n", resp.Total)
	b.WriteString("| Docket | Filer | Stance | Type | Position | Argument | Target party | Page | Conf |\n")
	b.WriteString("|--------|-------|--------|------|----------|----------|--------------|------|------|\n")
	for _, r := range resp.Results {
		conf := "-"
		if r.Confidence != nil {
			conf = strconv.FormatFloat(*r.Confidence, 'f', 2, 64)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			deref(r.DocketNumber),
			truncate(deref(r.Applicant), 30),
			deref(r.OverallStance),
			deref(r.ArgumentType),
			deref(r.Position),
			truncate(r.ArgumentText, 80),
			truncate(deref(r.TargetParty), 20),
			derefInt(r.SourcePage),
			conf,
		)
	}
	b.WriteString("\n_Arguments are LLM-extracted. Page numbers may be approximate. Use `get_filing_detail` with the filing ID for verbatim source quotes._\n")
	return b.String()
}

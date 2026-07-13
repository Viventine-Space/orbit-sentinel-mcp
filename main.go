package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is stamped by GoReleaser via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Println("orbit-sentinel-mcp " + version)
		return
	}

	_ = godotenv.Load() // optional — .env not required

	client := NewAPIClient()

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "orbit-sentinel",
			Version: version,
		},
		&mcp.ServerOptions{
			Instructions: `Orbit Sentinel: 418,000+ extracted space regulatory filings (946,000+ total) from FCC, ITU, UNOOSA, and FAA-AST.

## CRITICAL RULES — follow these without exception
1. ONLY state facts that appear in tool results. Never infer dates, names, acronyms, or relationships not in the data.
2. When a field shows "-" or is absent, say "not available in database" — do NOT guess or fill in from general knowledge.
3. Always cite the filing ID or entity ID when making claims about specific filings or entities.
4. If a search returns no results, say "No matching data found in the Orbit Sentinel database" — do NOT narrate from general knowledge.
5. Confidence levels (HIGH/MEDIUM/LOW) indicate extraction reliability. Flag LOW confidence data explicitly when presenting it.
6. This database does NOT cover: FCC experimental licenses (ELS), FCC terrestrial licenses (ULS), NOAA remote sensing licenses (CRSRA), or NTIA federal spectrum assignments. Say so when relevant.
7. Do not characterize a filing as someone's "first" or "only" filing unless the database search confirms no earlier filings exist.
8. Do not invent acronyms or abbreviations not present in the data.

## Tool Strategy (follow this order)
1. **Start with "research"** for any question — it searches filings, entities, and semantic text in parallel (one call instead of three).
2. Use **get_filing_detail** or **get_entity_profile** only when you have a specific UUID from research results and need full details.
3. Use **search_filings** or **search_entities** directly only when you need specific filter combinations (docket, agency+type, etc.) that research doesn't cover.
4. Use **search_semantic** directly only for targeted natural-language queries with specific filters (date range, agency).
5. Use **get_top_filers**, **get_filing_distribution**, or **get_filing_trends** for analytical questions about filing volumes, rankings, trends, or emerging players.

## Valid Filter Values
Agencies: FCC, ITU, UN_OOSA, FAA_AST, NOAA, NTIA, OFCOM_UK, CNES_FR, ACMA_AU, SEC, USPTO, OTHER
Filing types: SATELLITE_LICENSE, EARTH_STATION_PERMIT, EXPERIMENTAL_LICENSE, SPECIAL_TEMPORARY_AUTH, LAUNCH_LICENSE, REENTRY_LICENSE, REMOTE_SENSING_LICENSE, SPECTRUM_COORDINATION, ADVANCE_PUBLICATION, ORBITAL_DEBRIS_PLAN, ENVIRONMENTAL_ASSESSMENT, MODIFICATION, RENEWAL, TRANSFER_OF_CONTROL, COMMENT, OPPOSITION, EX_PARTE, PETITION, REGISTRATION, OTHER
Statuses: FILED, ACCEPTED, PUBLIC_NOTICE, COMMENT_PERIOD, UNDER_REVIEW, GRANTED, DENIED, DISMISSED, WITHDRAWN, EXPIRED, CANCELLED, UNKNOWN

## Tips
- Filing dates use YYYY-MM-DD format
- Entity search supports fuzzy name matching
- Semantic search uses cosine similarity (default threshold 0.5)
- Use count_only=true on search_filings when you just need a count`,
		},
	)

	registerTools(server, client)
	registerResources(server, client)
	registerPrompts(server, client)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

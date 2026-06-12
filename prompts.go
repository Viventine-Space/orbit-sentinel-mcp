package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPrompts(s *mcp.Server, client *APIClient) {
	s.AddPrompt(&mcp.Prompt{
		Name:        "regulatory_analysis",
		Description: "Analyze a specific regulatory filing for implications, technical parameters, and potential issues",
		Arguments: []*mcp.PromptArgument{
			{Name: "filing_id", Description: "UUID of the filing to analyze", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		filingID := req.Params.Arguments["filing_id"]
		if filingID == "" {
			return nil, fmt.Errorf("filing_id is required")
		}

		data, err := client.GetFiling(ctx, filingID)
		if err != nil {
			return nil, fmt.Errorf("fetch filing: %w", err)
		}
		filingText := formatFiling(data)

		prompt := fmt.Sprintf(`You are a space regulatory analyst examining a filing from the Orbit Sentinel database. Analyze the following filing and provide:

1. **Regulatory Context**: What type of authorization is being sought and from which agency? What regulatory framework applies?
2. **Technical Parameters**: Key spectrum allocations, orbital parameters, and ground station details. Are there any unusual or noteworthy technical aspects?
3. **Compliance Assessment**: Does the filing appear complete? Are there any missing elements that would typically be required?
4. **Potential Conflicts**: Based on the spectrum bands and orbital parameters, could this filing conflict with existing operations?
5. **Market Implications**: What does this filing suggest about the applicant's plans and competitive positioning?

IMPORTANT: Base your analysis ONLY on the data provided below. Do not add dates, names, acronyms, or facts from general knowledge. If a field is missing or shows "-", state it is not available in the database. Cite the filing ID when referencing this filing. Flag any LOW confidence extracted data explicitly.

---

%s`, filingText)

		return &mcp.GetPromptResult{
			Description: fmt.Sprintf("Regulatory analysis of filing %s", filingID),
			Messages: []*mcp.PromptMessage{
				{Role: "user", Content: &mcp.TextContent{Text: prompt}},
			},
		}, nil
	})

	s.AddPrompt(&mcp.Prompt{
		Name:        "competitive_landscape",
		Description: "Analyze an entity's regulatory filing history and competitive position in the space industry",
		Arguments: []*mcp.PromptArgument{
			{Name: "entity_name", Description: "Name of the entity to analyze (searched by fuzzy match)", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		entityName := req.Params.Arguments["entity_name"]
		if entityName == "" {
			return nil, fmt.Errorf("entity_name is required")
		}

		// Search for the entity by name
		data, err := client.SearchEntities(ctx, map[string]string{"q": entityName, "per_page": "5"})
		if err != nil {
			return nil, fmt.Errorf("search entities: %w", err)
		}
		entityListText := formatEntityList(data)

		// Pre-fetch the top entity's profile (same pattern as regulatory_analysis)
		var profileText string
		var env listEnvelope
		if json.Unmarshal(data, &env) == nil {
			var items []entityListItem
			if json.Unmarshal(env.Data, &items) == nil && len(items) > 0 {
				if profileData, err := client.GetEntityProfile(ctx, items[0].ID); err == nil {
					profileText = formatEntity(profileData)
				}
			}
		}

		var b strings.Builder
		fmt.Fprintf(&b, `You are a space industry competitive intelligence analyst using the Orbit Sentinel database. Analyze the entity "%s" using the data provided below.

Provide analysis covering:

1. **Entity Overview**: Who is this entity? What is their role in the space industry?
2. **Filing Activity**: How active are they in regulatory filings? What agencies do they file with most?
3. **Spectrum Holdings**: What frequency bands and orbital slots are they pursuing or holding?
4. **Recent Activity**: Any notable recent filings or changes in their regulatory strategy?
5. **Related Entities**: Who are their subsidiaries, partners, or frequent co-filers?
6. **Competitive Position**: How do they compare to competitors operating in similar spectrum bands or orbital regimes?

IMPORTANT: Base your analysis ONLY on the data provided below. Do not add dates, names, acronyms, or facts from general knowledge. If a field is missing or shows "-", state it is not available in the database. Cite the entity ID when referencing this entity. Flag any LOW confidence data explicitly.

---

Entity search results for "%s":

%s`, entityName, entityName, entityListText)

		if profileText != "" {
			fmt.Fprintf(&b, "\n---\n\nTop match profile:\n\n%s", profileText)
		}

		return &mcp.GetPromptResult{
			Description: fmt.Sprintf("Competitive landscape analysis for %s", entityName),
			Messages: []*mcp.PromptMessage{
				{Role: "user", Content: &mcp.TextContent{Text: b.String()}},
			},
		}, nil
	})
}

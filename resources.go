package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerResources(s *mcp.Server, client *APIClient) {
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "filing",
		Description: "A specific regulatory filing with full details",
		MIMEType:    "text/markdown",
		URITemplate: "filing://{id}",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := parseResourceID(req.Params.URI, "filing://")
		if id == "" {
			return nil, fmt.Errorf("invalid filing URI: %s", req.Params.URI)
		}
		data, err := client.GetFiling(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("fetch filing: %w", err)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: req.Params.URI, MIMEType: "text/markdown", Text: formatFiling(data)},
			},
		}, nil
	})

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "entity",
		Description: "A regulatory entity profile with filing history and relationships",
		MIMEType:    "text/markdown",
		URITemplate: "entity://{id}",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := parseResourceID(req.Params.URI, "entity://")
		if id == "" {
			return nil, fmt.Errorf("invalid entity URI: %s", req.Params.URI)
		}
		data, err := client.GetEntityProfile(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("fetch entity: %w", err)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: req.Params.URI, MIMEType: "text/markdown", Text: formatEntity(data)},
			},
		}, nil
	})
}

func parseResourceID(uri, prefix string) string {
	return strings.TrimPrefix(uri, prefix)
}

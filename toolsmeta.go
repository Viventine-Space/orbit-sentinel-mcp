package main

//go:generate go test -run TestToolsJSON -update

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// toolInfo is the public metadata for one MCP tool.
type toolInfo struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// parseRegisteredTools extracts MCP tool name+description metadata from the
// wrapAddTool(s, &mcp.Tool{...}, ...) registrations in the given source file, in
// source order. The inline registrations remain the single source of truth; this
// lets tools.json (consumed by the API /api/v1/meta catalog and the console docs)
// be generated from them so it can never drift. Kept fresh by TestToolsJSON.
func parseRegisteredTools(filename string) ([]toolInfo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}

	var tools []toolInfo
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "wrapAddTool" || len(call.Args) < 2 {
			return true
		}
		ue, ok := call.Args[1].(*ast.UnaryExpr) // &mcp.Tool{...}
		if !ok {
			return true
		}
		cl, ok := ue.X.(*ast.CompositeLit)
		if !ok {
			return true
		}
		var ti toolInfo
		for _, el := range cl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			lit, ok := kv.Value.(*ast.BasicLit)
			if !ok {
				continue
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				continue
			}
			switch key.Name {
			case "Name":
				ti.Name = val
			case "Description":
				ti.Desc = val
			}
		}
		if ti.Name != "" {
			tools = append(tools, ti)
		}
		return true
	})
	return tools, nil
}

# Orbit Sentinel MCP Server — AI Installation Guide

Step-by-step instructions for AI agents (Cline, Claude, etc.) installing this
MCP server on a user's machine.

## Overview

- **What it is**: stdio MCP server exposing 20 tools over 419,000+ space
  regulatory filings (FCC, ITU, UNOOSA, FAA-AST) via the Orbit Sentinel REST API.
- **Runtime**: single static Go binary. No Node, Python, or Docker required.
- **Credentials**: requires `MCP_API_KEY`. Free beta keys via magic-link signup
  at <https://console.viventine.com>.

## Step 1 — Get the API key

Ask the user for their Orbit Sentinel API key. If they don't have one, direct
them to <https://console.viventine.com> (magic-link email signup, free beta,
no credit card). Do not proceed without the key — the server starts without it
but every tool call will fail with an auth error.

## Step 2 — Download the binary

Pick the asset for the user's platform. `releases/latest/download/` URLs always
serve the newest version:

| Platform | URL |
|---|---|
| macOS Apple Silicon | `https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp_darwin_arm64.tar.gz` |
| macOS Intel | `https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp_darwin_amd64.tar.gz` |
| Linux x86_64 | `https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp_linux_amd64.tar.gz` |
| Linux arm64 | `https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp_linux_arm64.tar.gz` |
| Windows x86_64 | `https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp_windows_amd64.zip` |

macOS/Linux:

```bash
mkdir -p ~/.local/bin
curl -L https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp_darwin_arm64.tar.gz | tar -xz -C ~/.local/bin orbit-sentinel-mcp
```

macOS only — the binary is not notarized; clear the quarantine flag once:

```bash
xattr -d com.apple.quarantine ~/.local/bin/orbit-sentinel-mcp 2>/dev/null || true
```

Windows (PowerShell):

```powershell
Invoke-WebRequest https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp_windows_amd64.zip -OutFile orbit-sentinel-mcp.zip
Expand-Archive orbit-sentinel-mcp.zip -DestinationPath $env:LOCALAPPDATA\orbit-sentinel-mcp
```

Alternatives: `brew install --cask viventine-space/tap/orbit-sentinel-mcp`
(macOS) or `go install github.com/viventine-space/orbit-sentinel-mcp@latest`
(any platform with Go 1.25+).

## Step 3 — Verify the binary works

```bash
( echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"install-check","version":"1"}}}'; sleep 1 ) | ~/.local/bin/orbit-sentinel-mcp
```

A JSON response with `"name":"orbit-sentinel"` means the binary runs.

## Step 4 — Register the MCP server

Use the **absolute path** to the binary (`~` is not expanded by MCP clients).
Two environment variables:

- `MCP_API_KEY` (required) — the user's key from Step 1
- `MCP_API_URL` (required in practice) — the binary falls back to a localhost
  development URL when unset; always set it to
  `https://orbit-sentinel.viventine.com`

Standard MCP JSON block (Cline `cline_mcp_settings.json`, Claude Desktop
`claude_desktop_config.json`, Cursor `mcp.json` — same shape):

```json
{
  "mcpServers": {
    "orbit-sentinel": {
      "command": "/absolute/path/to/orbit-sentinel-mcp",
      "env": {
        "MCP_API_URL": "https://orbit-sentinel.viventine.com",
        "MCP_API_KEY": "USER_KEY_HERE"
      }
    }
  }
}
```

## Step 5 — Confirm

After the client reloads, 20 tools should be listed (`research`,
`search_filings`, `get_entity_profile`, `get_entity_dossier`,
`search_semantic`, `get_filing_trends`, …). Test with a call to `research`
using query `"Starlink"` — a JSON result list confirms the API key works.

## Troubleshooting

- **Tools listed but calls fail with 401/403**: wrong or missing `MCP_API_KEY`.
- **`killed` on first run (macOS)**: quarantine flag — rerun the `xattr` command.
- **No tools appear**: the configured `command` path is wrong; it must be absolute.

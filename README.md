# Orbit Sentinel MCP Server

[![CI](https://github.com/Viventine-Space/orbit-sentinel-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/Viventine-Space/orbit-sentinel-mcp/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Viventine-Space/orbit-sentinel-mcp)](https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest)
[![MCP Registry](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fregistry.modelcontextprotocol.io%2Fv0%2Fservers%2Fio.github.Viventine-Space%252Forbit-sentinel-mcp%2Fversions%2Flatest&query=%24.server.version&label=MCP%20Registry&prefix=v&color=blue)](https://registry.modelcontextprotocol.io/v0/servers/io.github.Viventine-Space%2Forbit-sentinel-mcp/versions/latest)
[![orbit-sentinel-mcp MCP server](https://glama.ai/mcp/servers/Viventine-Space/orbit-sentinel-mcp/badges/score.svg)](https://glama.ai/mcp/servers/Viventine-Space/orbit-sentinel-mcp)
[![Go Reference](https://pkg.go.dev/badge/github.com/viventine-space/orbit-sentinel-mcp.svg)](https://pkg.go.dev/github.com/viventine-space/orbit-sentinel-mcp)
[![License: MIT](https://img.shields.io/github/license/Viventine-Space/orbit-sentinel-mcp)](LICENSE)
[![MCP Badge](https://lobehub.com/badge/mcp/viventine-space-orbit-sentinel-mcp)](https://lobehub.com/mcp/viventine-space-orbit-sentinel-mcp)

MCP (Model Context Protocol) server for [Orbit Sentinel](https://viventine.com) —
419,000+ extracted space regulatory filings from FCC, ITU, UNOOSA, and FAA-AST,
queryable from Claude Desktop, Claude Code, Cursor, or any MCP client.

An API key is required. Beta access: <https://console.viventine.com>.

The server is a thin, open-source (MIT) stdio client of the public REST API —
five Go files, easy to audit before you run it.

## Install

### Claude Desktop (one-click)

Download [orbit-sentinel-mcp.mcpb](https://github.com/Viventine-Space/orbit-sentinel-mcp/releases/latest/download/orbit-sentinel-mcp.mcpb)
and double-click it — Claude Desktop installs the extension and prompts for
your API key. Bundles macOS (universal), Linux, and Windows binaries.

### Homebrew (macOS — recommended)

```bash
brew install --cask viventine-space/tap/orbit-sentinel-mcp
```

Installs to `$(brew --prefix)/bin/orbit-sentinel-mcp`, handles the quarantine
flag for you, and upgrades with `brew upgrade`.

### Manual download

Download the archive for your platform from the
[latest release](https://github.com/viventine-space/orbit-sentinel-mcp/releases/latest),
then:

```bash
tar -xzf orbit-sentinel-mcp_*.tar.gz
mkdir -p ~/bin && mv orbit-sentinel-mcp ~/bin/
```

macOS only — the binary is not notarized yet, so clear the quarantine flag once:

```bash
xattr -d com.apple.quarantine ~/bin/orbit-sentinel-mcp
```

Windows: unzip and note the full path to `orbit-sentinel-mcp.exe`.

### Build from source

```bash
go install github.com/viventine-space/orbit-sentinel-mcp@latest
```

## Configure

The server reads two environment variables:

| Variable | Purpose |
|---|---|
| `MCP_API_URL` | Orbit Sentinel API base URL (`https://orbit-sentinel.viventine.com`) |
| `MCP_API_KEY` | Your API key from the console |

**Claude Desktop** — add to `~/Library/Application Support/Claude/claude_desktop_config.json`
(Windows: `%APPDATA%\Claude\claude_desktop_config.json`), using the absolute
path to the binary (`~` is not expanded):

```json
{
  "mcpServers": {
    "orbit-sentinel": {
      "command": "/absolute/path/to/orbit-sentinel-mcp",
      "env": {
        "MCP_API_URL": "https://orbit-sentinel.viventine.com",
        "MCP_API_KEY": "<your-key>"
      }
    }
  }
}
```

**Claude Code** — one command:

```bash
claude mcp add orbit-sentinel \
  --env MCP_API_URL=https://orbit-sentinel.viventine.com \
  --env MCP_API_KEY=<your-key> \
  -- ~/bin/orbit-sentinel-mcp
```

Restart your client; the Orbit Sentinel tools (`research`, `search_filings`,
`get_entity_profile`, …) appear in the tools menu.

## Releasing (maintainers)

Tag and push — GitHub Actions builds and publishes all platforms:

```bash
git tag v0.x.y && git push origin v0.x.y
```

Asset names are version-stable (`orbit-sentinel-mcp_<os>_<arch>.tar.gz`), so
`releases/latest/download/...` URLs always serve the newest build.

Also rebuild and upload the Claude Desktop bundle so the `releases/latest`
one-click link stays valid: rebuild the three platform binaries into the MCPB
layout, `mcpb pack`, then `gh release upload v0.x.y orbit-sentinel-mcp.mcpb`
(candidate for goreleaser automation next cycle).

Then repeat the [Glama release](https://glama.ai/blog/2026-03-15-how-to-make-a-release)
— it does not auto-update from GitHub, and it gates the quality score on the
badge above. On the [Dockerfile admin page](https://glama.ai/mcp/servers/Viventine-Space/orbit-sentinel-mcp/admin/dockerfile),
the saved build spec should carry over (build steps install Go and `go build`;
placeholder parameters need a dummy `MCP_API_KEY` to satisfy the env schema) —
click **Build**, then **Make Release** with the new version. Manual for now;
consider Glama API integration next release cycle.

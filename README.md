# Orbit Sentinel MCP Server

MCP (Model Context Protocol) server for [Orbit Sentinel](https://viventine.com) —
418,000+ extracted space regulatory filings from FCC, ITU, UNOOSA, and FAA-AST,
queryable from Claude Desktop, Claude Code, Cursor, or any MCP client.

An API key is required. Beta access: <https://console.viventine.com>.

## Install

Download the archive for your platform from the
[latest release](https://github.com/acaracappa/orbit-sentinel-mcp/releases/latest),
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
go install github.com/acaracappa/orbit-sentinel-mcp@latest
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

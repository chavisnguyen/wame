#!/bin/bash
set -euo pipefail

# Register wame ("whatsapp" MCP server) into AGY, OpenCode, Claude Code, Kiro, Claude Desktop, and Codex.
# Usage:
#   ./install.sh sse    (Recommended for multiple agents: connects to http://localhost:3026/mcp)
#   ./install.sh stdio  (Standalone mode: spawns binary on demand)

MODE="${1:-sse}"
TOOL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
BIN_PATH="${WAME_BIN:-$TOOL_DIR/bin/wame}"
PORT="${PORT:-3026}"
SSE_URL="http://localhost:${PORT}/mcp"
SERVER_NAME="whatsapp"

if [ ! -f "$BIN_PATH" ] && [ -z "${WAME_BIN:-}" ]; then
    echo "⚙️ Building binary first..."
    (cd "$TOOL_DIR" && go build -o "$BIN_PATH" ./cmd/wame)
fi

echo "🔧 Configuring '$SERVER_NAME' MCP Server (wame) in [$MODE] mode..."
if [ "$MODE" = "sse" ]; then
    echo "   MCP URL: $SSE_URL"
else
    echo "   Binary:  $BIN_PATH (stdio mode)"
fi
echo ""

export MODE BIN_PATH SSE_URL SERVER_NAME

# --- 1. AGY: ~/.gemini/config/mcp_config.json ---
AGY_CONFIG="$HOME/.gemini/config/mcp_config.json"
echo "📝 Updating AGY config: $AGY_CONFIG"

python3 - << 'PYEOF'
import json, os

path = os.path.expanduser("~/.gemini/config/mcp_config.json")
mode = os.environ["MODE"]
sse_url = os.environ["SSE_URL"]
bin_path = os.environ["BIN_PATH"]
server_name = os.environ["SERVER_NAME"]

if os.path.exists(path):
    with open(path) as f:
        try:
            data = json.load(f)
        except Exception:
            data = {}
else:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    data = {}

data.setdefault("mcpServers", {})
if mode == "sse":
    data["mcpServers"][server_name] = {
        "disabled": False,
        "serverUrl": sse_url
    }
else:
    data["mcpServers"][server_name] = {
        "command": bin_path,
        "args": ["serve"]
    }

with open(path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")

print("   ✅ AGY config updated")
PYEOF

# --- 2. OpenCode: ~/.config/opencode/opencode.json(c) ---
echo "📝 Updating OpenCode config..."

python3 - << 'PYEOF'
import json, os, re

base = os.path.expanduser("~/.config/opencode")
candidates = [os.path.join(base, "opencode.jsonc"), os.path.join(base, "opencode.json")]
path = next((p for p in candidates if os.path.exists(p)), candidates[1])

mode = os.environ["MODE"]
sse_url = os.environ["SSE_URL"]
bin_path = os.environ["BIN_PATH"]
server_name = os.environ["SERVER_NAME"]

def strip_jsonc(text):
    text = re.sub(r"/\*.*?\*/", "", text, flags=re.DOTALL)
    text = re.sub(r"(^|\s)//[^\n]*", r"\1", text)
    text = re.sub(r",(\s*[}\]])", r"\1", text)
    return text

if os.path.exists(path):
    with open(path) as f:
        raw = f.read()
    data = json.loads(strip_jsonc(raw)) if raw.strip() else {}
else:
    os.makedirs(base, exist_ok=True)
    data = {}

data.setdefault("mcp", {}).setdefault("servers", {})
if mode == "sse":
    data["mcp"]["servers"][server_name] = {
        "type": "remote",
        "url": sse_url
    }
else:
    data["mcp"]["servers"][server_name] = {
        "type": "stdio",
        "command": bin_path,
        "args": ["serve"]
    }

with open(path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")

print(f"   ✅ OpenCode config updated ({os.path.basename(path)})")
PYEOF

# --- 3. Claude Code (CLI): ~/.claude.json ---
CLAUDE_CLI_CONFIG="$HOME/.claude.json"
echo "📝 Updating Claude Code (CLI) config: $CLAUDE_CLI_CONFIG"

python3 - << 'PYEOF'
import json, os

path = os.path.expanduser("~/.claude.json")
mode = os.environ["MODE"]
sse_url = os.environ["SSE_URL"]
bin_path = os.environ["BIN_PATH"]
server_name = os.environ["SERVER_NAME"]

if os.path.exists(path):
    with open(path) as f:
        try:
            data = json.load(f)
        except Exception:
            data = {}
else:
    data = {}

data.setdefault("mcpServers", {})
if mode == "sse":
    data["mcpServers"][server_name] = {
        "type": "http",
        "url": sse_url
    }
else:
    data["mcpServers"][server_name] = {
        "command": bin_path,
        "args": ["serve"]
    }

with open(path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")

print("   ✅ Claude Code (CLI) config updated")
PYEOF

# --- 4. Kiro: ~/.kiro/settings/mcp.json ---
KIRO_CONFIG="$HOME/.kiro/settings/mcp.json"
echo "📝 Updating Kiro config: $KIRO_CONFIG"

python3 - << 'PYEOF'
import json, os

path = os.path.expanduser("~/.kiro/settings/mcp.json")
mode = os.environ["MODE"]
sse_url = os.environ["SSE_URL"]
bin_path = os.environ["BIN_PATH"]
server_name = os.environ["SERVER_NAME"]

if os.path.exists(path):
    with open(path) as f:
        try:
            data = json.load(f)
        except Exception:
            data = {}
else:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    data = {}

data.setdefault("mcpServers", {})
if mode == "sse":
    data["mcpServers"][server_name] = {
        "url": sse_url,
        "disabled": False
    }
else:
    data["mcpServers"][server_name] = {
        "command": bin_path,
        "args": ["serve"],
        "disabled": False
    }

with open(path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")

print("   ✅ Kiro config updated")
PYEOF

# --- 5. Claude Desktop (requires stdio): ~/Library/Application Support/Claude/claude_desktop_config.json ---
CLAUDE_DIR="$HOME/Library/Application Support/Claude"
CLAUDE_CONFIG="$CLAUDE_DIR/claude_desktop_config.json"
if [ -d "$CLAUDE_DIR" ]; then
    echo "📝 Updating Claude Desktop config: $CLAUDE_CONFIG"
    python3 - << 'PYEOF'
import json, os

path = os.path.expanduser("~/Library/Application Support/Claude/claude_desktop_config.json")
bin_path = os.environ["BIN_PATH"]
server_name = os.environ["SERVER_NAME"]

if os.path.exists(path):
    with open(path) as f:
        try:
            data = json.load(f)
        except Exception:
            data = {}
else:
    data = {}

data.setdefault("mcpServers", {})
data["mcpServers"][server_name] = {
    "command": bin_path,
    "args": ["serve"]
}

with open(path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")

print("   ✅ Claude Desktop config updated (stdio)")
PYEOF
fi

# --- 6. Codex: $CODEX_HOME/config.toml (default ~/.codex/config.toml) ---
CODEX_DIR="${CODEX_HOME:-$HOME/.codex}"
CODEX_CONFIG="$CODEX_DIR/config.toml"
echo "📝 Updating Codex config: $CODEX_CONFIG"

CODEX_CONFIG="$CODEX_CONFIG" python3 - << 'PYEOF'
import json
import os
import re

path = os.path.expanduser(os.environ["CODEX_CONFIG"])
mode = os.environ["MODE"]
sse_url = os.environ["SSE_URL"]
bin_path = os.environ["BIN_PATH"]
server_name = os.environ["SERVER_NAME"]

os.makedirs(os.path.dirname(path), exist_ok=True)

if os.path.exists(path):
    with open(path, "r", encoding="utf-8") as f:
        raw = f.read()
else:
    raw = ""

target_names = {server_name}

def parse_table_header(line):
    stripped = line.strip()
    if stripped.startswith("[[") and "]]" in stripped:
        inner = stripped[2:stripped.index("]]")].strip()
        return inner, True
    if stripped.startswith("[") and "]" in stripped and not stripped.startswith("[["):
        inner = stripped[1:stripped.index("]")].strip()
        return inner, False
    return None, False

def normalize_part(part):
    part = part.strip()
    if (part.startswith('"') and part.endswith('"')) or (part.startswith("'") and part.endswith("'")):
        return part[1:-1]
    return part

def split_toml_path(header):
    parts = []
    cur = []
    quote = None
    escaped = False
    for ch in header:
        if escaped:
            cur.append(ch)
            escaped = False
            continue
        if quote == '"' and ch == "\\":
            cur.append(ch)
            escaped = True
            continue
        if ch in ('"', "'"):
            if quote is None:
                quote = ch
            elif quote == ch:
                quote = None
            cur.append(ch)
        elif ch == "." and quote is None:
            parts.append(normalize_part("".join(cur)))
            cur = []
        else:
            cur.append(ch)
    if cur:
        parts.append(normalize_part("".join(cur)))
    return parts

def is_target_mcp_table(parts):
    return len(parts) >= 2 and parts[0] == "mcp_servers" and parts[1] in target_names

def is_mcp_servers_root(parts):
    return len(parts) == 1 and parts[0] == "mcp_servers"

lines = raw.splitlines(keepends=True)
out_lines = []
skip_section = False
in_mcp_root = False

inline_key_re = re.compile(r'^\s*(?:"([^"]+)"|\'([^\']+)\'|([A-Za-z0-9_-]+))\s*=')

for line in lines:
    header, _ = parse_table_header(line)
    if header is not None:
        parts = split_toml_path(header)
        skip_section = is_target_mcp_table(parts)
        in_mcp_root = is_mcp_servers_root(parts)
        if not skip_section:
            out_lines.append(line)
        continue

    if skip_section:
        continue

    if in_mcp_root:
        m = inline_key_re.match(line)
        if m:
            key = m.group(1) or m.group(2) or m.group(3)
            if key in target_names:
                continue

    out_lines.append(line)

while out_lines and out_lines[-1].strip() == "":
    out_lines.pop()

body = "".join(out_lines)
if body and not body.endswith("\n"):
    body += "\n"
if body:
    body += "\n"

if mode == "sse":
    new_block = f"[mcp_servers.{server_name}]\nurl = {json.dumps(sse_url)}\n"
else:
    new_block = f"[mcp_servers.{server_name}]\ncommand = {json.dumps(bin_path)}\nargs = [\"serve\"]\n"

updated = body + new_block

try:
    import tomllib
    tomllib.loads(updated)
except ModuleNotFoundError:
    pass

with open(path, "w", encoding="utf-8") as f:
    f.write(updated)

print("   ✅ Codex config updated")
PYEOF

echo ""
echo "🎉 Done! '$SERVER_NAME' MCP Server (wame) configured in [$MODE] mode across your AI tools."
if [ "$MODE" = "sse" ] && [ -z "${WAME_BIN:-}" ]; then
    echo "💡 Note: Run 'make daemon' (or 'brew services start wame') to start the background server."
fi

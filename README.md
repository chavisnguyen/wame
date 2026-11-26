# wame

> Let your AI agent (Claude Code, Codex, OpenCode, AGY, Kiro, Claude Desktop) use **your personal WhatsApp** via MCP.

[![Release](https://img.shields.io/github/v/release/chavisnguyen/wame?style=flat-square&color=blue)](https://github.com/chavisnguyen/wame/releases)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![MCP](https://img.shields.io/badge/MCP-Compatible-blue?style=flat-square)](https://modelcontextprotocol.io)
[![RAM](https://img.shields.io/badge/RAM-~15MB-brightgreen?style=flat-square)](README.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)

Lightweight WhatsApp [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server written in Go. Connects AI agents directly to your personal WhatsApp account without Node or Python runtime dependencies.

---

## Install (macOS / Linux)

### Option A: Homebrew (Recommended)

```bash
brew tap chavisnguyen/wame https://github.com/chavisnguyen/wame
brew install wame
wame setup                 # Prepare ~/.wame/store
wame login                 # Scan QR code in terminal (one-time)
wame service start         # Or: brew services start wame (runs on :3026)
wame doctor                # Verify store, session, and daemon
```

Register `http://localhost:3026/mcp` in your AI coding tools via [`mcps`](https://github.com/chavisnguyen/mcps) (`mcps add whatsapp --url http://localhost:3026/mcp`).

### Option B: Build from source

```bash
git clone https://github.com/chavisnguyen/wame.git
cd wame

make setup   # Build bin/wame and register "whatsapp" MCP in detected AI clients
make login   # Scan QR code in terminal (one-time)
make daemon  # Start background shared server on :3026
make doctor  # Verify connection and configuration health
```

---

## Features

- **Pure Go (~15MB RAM):** Single binary powered by `whatsmeow` and `mcp-go`.
- **Shared Daemon (`:3026`):** One background process keeps your WhatsApp session alive while serving multiple AI clients concurrently (`http://localhost:3026/mcp`) without session drops or SQLite locks.
- **Persistent Store (`~/.wame/store`):** Works reliably regardless of working directory or Homebrew install location (automatically migrates data from legacy `./store` on first run).
- **1-Command Client Setup (`wame setup`):** Registers the `"whatsapp"` MCP server across **AGY**, **OpenCode**, **Claude Code**, **Kiro**, **Codex**, and **Claude Desktop** (stdio).
- **Loopback & Origin Protection:** Binds to loopback (`127.0.0.1` & `[::1]`) and rejects unauthorized browser `Origin` headers on `/mcp`, `/sse`, and `/health`.
- **Voice & Media Support:** Sends and downloads photos, videos, documents, and PTT voice notes (via FFmpeg).

---

## Commands

| Command | Action |
| :--- | :--- |
| `wame setup` | Prepare `~/.wame/store` and register `"whatsapp"` MCP in all AI clients |
| `wame login` | Pair WhatsApp account via terminal QR code |
| `wame daemon` | Run shared HTTP/SSE daemon in foreground (`-port 3026`) |
| `wame serve` | Run stdio MCP server (used by Claude Desktop / stdio clients) |
| `wame status` | Check WhatsApp authentication & store path |
| `wame doctor` | Run full system diagnostic (`🟢/🟡/🔴`) |
| `wame version` | Print version (also `-v`, `--version`) |

Source-checkout helpers (`Makefile`): `make setup | login | daemon | stop | restart | status | doctor | update | test | clean | help`.

---

## Configuration (`.env` / env vars)

Copy `.env.example` to `.env` (or `~/.wame/.env`):

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `3026` | HTTP/SSE daemon port |
| `HOST` | *(loopback)* | Specific bind host (empty binds `127.0.0.1` and `[::1]`) |
| `WAME_STORE_DIR` | `~/.wame/store` | Directory for `whatsapp.db`, `messages.db`, and downloaded media |
| `WAME_ALLOWED_ORIGINS` | *(empty)* | Comma-separated browser `Origin` allowlist (CLI clients need none) |

---

## Manual Client Config

### HTTP/SSE Shared Mode (Recommended)
```json
{
  "mcpServers": {
    "whatsapp": {
      "url": "http://localhost:3026/mcp"
    }
  }
}
```

### stdio Mode (Claude Desktop / Direct binary)
```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "wame",
      "args": ["serve"]
    }
  }
}
```

---

## MCP Tools (13)

| Tool | Description |
| :--- | :--- |
| `get_whatsapp_status` | Check connection health, account info, and database stats |
| `search_contacts` | Search contacts by name or phone number |
| `list_messages` | Query messages with filters (time, sender, chat, query, context) |
| `list_chats` | List recent conversations and group chats |
| `get_chat` | Get chat metadata by JID |
| `get_direct_chat_by_contact` | Find chat by phone number |
| `get_contact_chats` | List all chats involving a specific contact |
| `get_last_interaction` | Get most recent message with a contact |
| `get_message_context` | Expand context before and after a specific message |
| `send_message` | Send text message to phone number or group JID |
| `send_file` | Send image, video, document, or audio file |
| `send_audio_message` | Send audio file formatted as WhatsApp voice memo |
| `download_media` | Download encrypted media attachment to local folder |

---

## License

[MIT](LICENSE)

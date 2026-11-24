package wame

import _ "embed"

// InstallScript is the embedded setup script that registers the "whatsapp" MCP server across supported AI clients.
//
//go:embed install.sh
var InstallScript []byte

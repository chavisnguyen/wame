package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"wame/internal/store"
)

func TestMCPServer_ToolsRegistrationAndExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wa-mcp-server-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	msgStore, err := store.NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}
	defer msgStore.Close()

	// Seed some test data
	now := time.Now()
	chatJID := "987654321@s.whatsapp.net"
	_ = msgStore.StoreChat(chatJID, "Bob Dylan", now)
	_ = msgStore.StoreMessage(
		"MSG_A", chatJID, "987654321", "The answer my friend is blowin in the wind",
		now.Add(-1*time.Minute), false, "", "", "", nil, nil, nil, 0,
	)

	// Create MCP Server without live WhatsApp client (nil for offline testing)
	s := NewServer(nil, msgStore)

	ctx := context.Background()

	// 1. Test search_contacts
	res, err := s.handleSearchContacts(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search_contacts",
			Arguments: map[string]interface{}{
				"query": "Dylan",
			},
		},
	})
	if err != nil {
		t.Fatalf("handleSearchContacts error: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("Expected content in result")
	}
	textResult := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(textResult, "Bob Dylan") {
		t.Errorf("Expected Bob Dylan in result, got: %s", textResult)
	}

	// 2. Test list_messages
	res, err = s.handleListMessages(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "list_messages",
			Arguments: map[string]interface{}{
				"chat_jid": chatJID,
			},
		},
	})
	if err != nil {
		t.Fatalf("handleListMessages error: %v", err)
	}
	textResult = res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(textResult, "blowin in the wind") {
		t.Errorf("Expected message content in result, got: %s", textResult)
	}

	// 3. Test list_chats
	res, err = s.handleListChats(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "list_chats",
			Arguments: map[string]interface{}{
				"query": "Bob",
			},
		},
	})
	if err != nil {
		t.Fatalf("handleListChats error: %v", err)
	}
	textResult = res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(textResult, "Bob Dylan") {
		t.Errorf("Expected Bob Dylan in chats, got: %s", textResult)
	}

	// 4. Test get_chat
	res, err = s.handleGetChat(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_chat",
			Arguments: map[string]interface{}{
				"chat_jid": chatJID,
			},
		},
	})
	if err != nil {
		t.Fatalf("handleGetChat error: %v", err)
	}
	textResult = res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(textResult, chatJID) {
		t.Errorf("Expected chat JID in result, got: %s", textResult)
	}

	// 5. Test get_last_interaction
	res, err = s.handleGetLastInteraction(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_last_interaction",
			Arguments: map[string]interface{}{
				"jid": chatJID,
			},
		},
	})
	if err != nil {
		t.Fatalf("handleGetLastInteraction error: %v", err)
	}
	textResult = res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(textResult, "blowin in the wind") {
		t.Errorf("Expected interaction message in result, got: %s", textResult)
	}
}

func TestBuildMediaToolResult_ImageTextAndBinary(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wa-mcp-media-result-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Minimal 1x1 PNG image -> should return TextContent + ImageContent
	pngB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	pngBytes, _ := base64.StdEncoding.DecodeString(pngB64)
	imgPath := filepath.Join(tempDir, "sample.png")
	if err := os.WriteFile(imgPath, pngBytes, 0644); err != nil {
		t.Fatalf("WriteFile png failed: %v", err)
	}

	imgRes := buildMediaToolResult(imgPath, &store.MediaInfo{
		MediaType: "image",
		Filename:  "sample.png",
		MimeType:  "image/png",
	}, true)

	if len(imgRes.Content) != 2 {
		t.Fatalf("Expected 2 content items (text + image), got %d", len(imgRes.Content))
	}
	imgContent, ok := mcp.AsImageContent(imgRes.Content[1])
	if !ok {
		t.Fatalf("Expected second content item to be ImageContent, got %T", imgRes.Content[1])
	}
	if imgContent.MIMEType != "image/png" || imgContent.Data != pngB64 {
		t.Errorf("Unexpected ImageContent: mime=%s dataLen=%d", imgContent.MIMEType, len(imgContent.Data))
	}

	// With includeContent=false -> text metadata only
	metaOnlyRes := buildMediaToolResult(imgPath, &store.MediaInfo{MediaType: "image", Filename: "sample.png"}, false)
	if len(metaOnlyRes.Content) != 1 {
		t.Errorf("Expected 1 content item when includeContent=false, got %d", len(metaOnlyRes.Content))
	}

	// 2. UTF-8 text/CSV document -> should include inline text_content
	csvPath := filepath.Join(tempDir, "report.csv")
	csvBody := "name,amount\nAlice,1500\nBob,2300\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0644); err != nil {
		t.Fatalf("WriteFile csv failed: %v", err)
	}

	txtRes := buildMediaToolResult(csvPath, &store.MediaInfo{
		MediaType: "document",
		Filename:  "report.csv",
		MimeType:  "text/csv",
	}, true)
	if len(txtRes.Content) != 1 {
		t.Fatalf("Expected 1 text content item for CSV, got %d", len(txtRes.Content))
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(txtRes.Content[0].(mcp.TextContent).Text), &parsed); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
	if parsed["text_content"] != csvBody {
		t.Errorf("Expected inline text_content %q, got %v", csvBody, parsed["text_content"])
	}

	// 3. Binary PDF -> metadata only without text_content
	pdfPath := filepath.Join(tempDir, "doc.pdf")
	_ = os.WriteFile(pdfPath, []byte("%PDF-1.7\n\x00\xff\xfe"), 0644)
	pdfRes := buildMediaToolResult(pdfPath, &store.MediaInfo{
		MediaType: "document",
		Filename:  "doc.pdf",
		MimeType:  "application/pdf",
	}, true)
	var pdfParsed map[string]interface{}
	_ = json.Unmarshal([]byte(pdfRes.Content[0].(mcp.TextContent).Text), &pdfParsed)
	if _, hasText := pdfParsed["text_content"]; hasText {
		t.Errorf("Did not expect text_content for binary PDF")
	}
	if pdfParsed["mime_type"] != "application/pdf" {
		t.Errorf("Expected application/pdf mime_type, got %v", pdfParsed["mime_type"])
	}
}

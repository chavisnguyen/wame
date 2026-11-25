package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStore_AllOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wa-mcp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer s.Close()

	now := time.Now().Truncate(time.Second)

	// 1. Test StoreChat
	chatJID := "1234567890@s.whatsapp.net"
	chatName := "Alice Smith"
	err = s.StoreChat(chatJID, chatName, now)
	if err != nil {
		t.Fatalf("StoreChat failed: %v", err)
	}

	// 2. Test GetSenderName
	resolvedName := s.GetSenderName(chatJID)
	if resolvedName != chatName {
		t.Errorf("Expected %s, got %s", chatName, resolvedName)
	}

	// 3. Test StoreMessage
	msgID1 := "MSG_001"
	err = s.StoreMessage(
		msgID1, chatJID, "1234567890", "Hello there!", now.Add(-2*time.Minute), false,
		"", "", "", nil, nil, nil, 0,
	)
	if err != nil {
		t.Fatalf("StoreMessage 1 failed: %v", err)
	}

	msgID2 := "MSG_002"
	err = s.StoreMessage(
		msgID2, chatJID, "me", "Hi Alice!", now.Add(-1*time.Minute), true,
		"", "", "", nil, nil, nil, 0,
	)
	if err != nil {
		t.Fatalf("StoreMessage 2 failed: %v", err)
	}

	msgID3 := "MSG_003"
	mediaKey := []byte{1, 2, 3, 4}
	fileSHA := []byte{5, 6, 7, 8}
	fileEncSHA := []byte{9, 10, 11, 12}
	err = s.StoreMessage(
		msgID3, chatJID, "1234567890", "Check this picture", now, false,
		"image", "photo.jpg", "https://mmg.whatsapp.net/v/photo.enc", mediaKey, fileSHA, fileEncSHA, 1024,
	)
	if err != nil {
		t.Fatalf("StoreMessage 3 failed: %v", err)
	}

	// 4. Test SearchContacts
	contacts, err := s.SearchContacts("Alice")
	if err != nil {
		t.Fatalf("SearchContacts failed: %v", err)
	}
	if len(contacts) != 1 || contacts[0].Name != chatName {
		t.Errorf("Expected 1 contact named Alice, got %v", contacts)
	}

	// 5. Test ListMessages without context
	msgs, err := s.ListMessages(ListMessagesFilter{
		ChatJID: chatJID,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(msgs))
	}

	// 6. Test ListMessages with Query
	queryMsgs, err := s.ListMessages(ListMessagesFilter{
		Query: "Alice",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListMessages with query failed: %v", err)
	}
	if len(queryMsgs) != 1 || queryMsgs[0].ID != msgID2 {
		t.Errorf("Expected message MSG_002, got %v", queryMsgs)
	}

	// 7. Test GetMessageContext
	ctx, err := s.GetMessageContext(msgID2, 1, 1)
	if err != nil {
		t.Fatalf("GetMessageContext failed: %v", err)
	}
	if ctx.Message.ID != msgID2 {
		t.Errorf("Expected target message ID MSG_002, got %s", ctx.Message.ID)
	}
	if len(ctx.Before) != 1 || ctx.Before[0].ID != msgID1 {
		t.Errorf("Expected 1 message before (MSG_001), got %v", ctx.Before)
	}
	if len(ctx.After) != 1 || ctx.After[0].ID != msgID3 {
		t.Errorf("Expected 1 message after (MSG_003), got %v", ctx.After)
	}

	// 8. Test ListChats
	chats, err := s.ListChats("", 10, 0, true, "last_active")
	if err != nil {
		t.Fatalf("ListChats failed: %v", err)
	}
	if len(chats) != 1 || chats[0].JID != chatJID {
		t.Errorf("Expected 1 chat with JID %s, got %v", chatJID, chats)
	}

	// 9. Test GetChat
	c, err := s.GetChat(chatJID, true)
	if err != nil || c == nil {
		t.Fatalf("GetChat failed: %v", err)
	}
	if c.Name != chatName {
		t.Errorf("Expected chat name %s, got %s", chatName, c.Name)
	}

	// 10. Test GetDirectChatByContact
	directChat, err := s.GetDirectChatByContact("1234567890")
	if err != nil || directChat == nil {
		t.Fatalf("GetDirectChatByContact failed: %v", err)
	}
	if directChat.JID != chatJID {
		t.Errorf("Expected JID %s, got %s", chatJID, directChat.JID)
	}

	// 11. Test GetLastInteraction
	lastMsg, err := s.GetLastInteraction(chatJID)
	if err != nil || lastMsg == nil {
		t.Fatalf("GetLastInteraction failed: %v", err)
	}
	if lastMsg.ID != msgID3 {
		t.Errorf("Expected last message MSG_003, got %s", lastMsg.ID)
	}

	// 12. Test GetMediaInfo
	mediaInfo, err := s.GetMediaInfo(msgID3, chatJID)
	if err != nil {
		t.Fatalf("GetMediaInfo failed: %v", err)
	}
	if mediaInfo.MediaType != "image" || mediaInfo.Filename != "photo.jpg" {
		t.Errorf("Expected image photo.jpg, got %v", mediaInfo)
	}

	// 13. Test StoreMessageWithMeta, filename query, and media preview in ListChats
	msgID4 := "MSG_004"
	err = s.StoreMessageWithMeta(
		msgID4, chatJID, "1234567890", "", now.Add(1*time.Minute), false,
		"document", "contract_2026.pdf", "application/pdf", "", "/v/t62.7119-24/doc.enc",
		mediaKey, fileSHA, fileEncSHA, 8192,
	)
	if err != nil {
		t.Fatalf("StoreMessageWithMeta failed: %v", err)
	}
	_ = s.StoreChat(chatJID, chatName, now.Add(1*time.Minute))

	metaInfo, err := s.GetMediaInfo(msgID4, chatJID)
	if err != nil {
		t.Fatalf("GetMediaInfo MSG_004 failed: %v", err)
	}
	if metaInfo.MimeType != "application/pdf" || metaInfo.DirectPath != "/v/t62.7119-24/doc.enc" {
		t.Errorf("Expected mime_type and direct_path preserved, got %+v", metaInfo)
	}

	fileMatches, err := s.ListMessages(ListMessagesFilter{Query: "contract_2026.pdf", Limit: 10})
	if err != nil || len(fileMatches) != 1 || fileMatches[0].ID != msgID4 {
		t.Errorf("Expected filename search to find MSG_004, got %+v (err=%v)", fileMatches, err)
	}

	formatted := FormatMessage(fileMatches[0], chatName, true)
	if !strings.Contains(formatted, "[document: contract_2026.pdf") {
		t.Errorf("Expected filename in formatted message, got %s", formatted)
	}

	updatedChat, err := s.GetChat(chatJID, true)
	if err != nil || updatedChat == nil {
		t.Fatalf("GetChat after media failed: %v", err)
	}
	if updatedChat.LastMessage != "[document: contract_2026.pdf]" {
		t.Errorf("Expected media preview '[document: contract_2026.pdf]', got %q", updatedChat.LastMessage)
	}
}

func TestStore_LegacySchemaMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wa-mcp-legacy-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "messages.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	_, err = rawDB.Exec(`
		CREATE TABLE chats (jid TEXT PRIMARY KEY, name TEXT, last_message_time TIMESTAMP);
		CREATE TABLE messages (
			id TEXT, chat_jid TEXT, sender TEXT, content TEXT, timestamp TIMESTAMP,
			is_from_me BOOLEAN, media_type TEXT, filename TEXT, url TEXT,
			media_key BLOB, file_sha256 BLOB, file_enc_sha256 BLOB, file_length INTEGER,
			PRIMARY KEY (id, chat_jid)
		);
	`)
	if err != nil {
		t.Fatalf("Failed creating legacy schema: %v", err)
	}
	rawDB.Close()

	s, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore migration on legacy DB failed: %v", err)
	}
	defer s.Close()

	now := time.Now()
	_ = s.StoreChat("111@s.whatsapp.net", "Legacy User", now)
	if err := s.StoreMessageWithMeta(
		"L1", "111@s.whatsapp.net", "111", "hi", now, false,
		"image", "img.png", "image/png", "", "/v/direct", []byte{1}, nil, nil, 10,
	); err != nil {
		t.Fatalf("StoreMessageWithMeta after migration failed: %v", err)
	}
}

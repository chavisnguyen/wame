package whatsapp

import (
	"os"
	"strings"
	"testing"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

func TestExtractDirectPathFromURL(t *testing.T) {
	url := "https://mmg.whatsapp.net/v/t62.7118-24/13812002_698058036224062_3424455886509161511_n.enc?ccb=11-4&oh=abc"
	directPath := ExtractDirectPathFromURL(url)
	expected := "/v/t62.7118-24/13812002_698058036224062_3424455886509161511_n.enc"
	if directPath != expected {
		t.Errorf("Expected %s, got %s", expected, directPath)
	}
}

func TestExtractTextContent(t *testing.T) {
	msg1 := &waProto.Message{
		Conversation: proto.String("Simple conversation"),
	}
	if text := ExtractTextContent(msg1); text != "Simple conversation" {
		t.Errorf("Expected 'Simple conversation', got '%s'", text)
	}

	msg2 := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String("Extended message text"),
		},
	}
	if text := ExtractTextContent(msg2); text != "Extended message text" {
		t.Errorf("Expected 'Extended message text', got '%s'", text)
	}
}

func TestExtractMediaInfo(t *testing.T) {
	imgMsg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String("https://mmg.whatsapp.net/v/test.enc"),
			MediaKey:      []byte{1, 2, 3},
			FileSHA256:    []byte{4, 5, 6},
			FileEncSHA256: []byte{7, 8, 9},
			FileLength:    proto.Uint64(2048),
		},
	}

	mediaType, filename, url, mediaKey, sha, encSha, length := ExtractMediaInfo(imgMsg)
	if mediaType != "image" {
		t.Errorf("Expected mediaType 'image', got '%s'", mediaType)
	}
	if filename == "" {
		t.Errorf("Expected non-empty filename")
	}
	if url != "https://mmg.whatsapp.net/v/test.enc" || length != 2048 {
		t.Errorf("Unexpected media info values: url=%s, length=%d", url, length)
	}
	if len(mediaKey) != 3 || len(sha) != 3 || len(encSha) != 3 {
		t.Errorf("Unexpected byte slice lengths")
	}
}

func TestGeneratePlaceholderWaveform(t *testing.T) {
	waveform := generatePlaceholderWaveform(10)
	if len(waveform) != 64 {
		t.Errorf("Expected waveform length 64, got %d", len(waveform))
	}
	for i, v := range waveform {
		if v > 100 {
			t.Errorf("Waveform byte %d exceeds 100: %d", i, v)
		}
	}
}

func TestUnwrapAndExtractMediaDetails(t *testing.T) {
	ts := time.Date(2026, 11, 25, 17, 0, 0, 0, time.UTC)

	// 1. Ephemeral -> ViewOnceV2 -> ImageMessage with caption, PNG mime, DirectPath
	wrappedImg := &waProto.Message{
		EphemeralMessage: &waProto.FutureProofMessage{
			Message: &waProto.Message{
				ViewOnceMessageV2: &waProto.FutureProofMessage{
					Message: &waProto.Message{
						ImageMessage: &waProto.ImageMessage{
							Caption:       proto.String("Screenshot error"),
							Mimetype:      proto.String("image/png"),
							DirectPath:    proto.String("/v/t62.7118-24/direct_img.enc"),
							MediaKey:      []byte{1, 2, 3},
							FileSHA256:    []byte{4, 5, 6},
							FileEncSHA256: []byte{7, 8, 9},
							FileLength:    proto.Uint64(4096),
						},
					},
				},
			},
		},
	}

	if caption := ExtractTextContent(wrappedImg); caption != "Screenshot error" {
		t.Errorf("Expected caption 'Screenshot error', got %q", caption)
	}

	detailsA := ExtractMediaDetails(wrappedImg, "MSG_AAA", ts)
	detailsB := ExtractMediaDetails(wrappedImg, "MSG_BBB", ts)
	if detailsA.MediaType != "image" || detailsA.MimeType != "image/png" {
		t.Errorf("Unexpected image details: %+v", detailsA)
	}
	if !strings.HasSuffix(detailsA.Filename, ".png") {
		t.Errorf("Expected .png filename, got %q", detailsA.Filename)
	}
	if detailsA.Filename == detailsB.Filename {
		t.Errorf("Expected distinct filenames for distinct message IDs in same second, got %q", detailsA.Filename)
	}
	if detailsA.DirectPath != "/v/t62.7118-24/direct_img.enc" {
		t.Errorf("Expected DirectPath preserved, got %q", detailsA.DirectPath)
	}

	// 2. DocumentWithCaptionMessage -> DocumentMessage
	wrappedDoc := &waProto.Message{
		DocumentWithCaptionMessage: &waProto.FutureProofMessage{
			Message: &waProto.Message{
				DocumentMessage: &waProto.DocumentMessage{
					FileName:   proto.String("invoice_nov.pdf"),
					Caption:    proto.String("Please review"),
					Mimetype:   proto.String("application/pdf"),
					DirectPath: proto.String("/v/t62.7119-24/doc.enc"),
					MediaKey:   []byte{9, 9, 9},
				},
			},
		},
	}

	if caption := ExtractTextContent(wrappedDoc); caption != "Please review" {
		t.Errorf("Expected document caption 'Please review', got %q", caption)
	}
	docDetails := ExtractMediaDetails(wrappedDoc, "MSG_DOC1", ts)
	if docDetails.MediaType != "document" || docDetails.Filename != "invoice_nov.pdf" || docDetails.MimeType != "application/pdf" {
		t.Errorf("Unexpected document details: %+v", docDetails)
	}

	// 3. StickerMessage
	stickerMsg := &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			Mimetype:   proto.String("image/webp"),
			DirectPath: proto.String("/v/sticker.enc"),
			MediaKey:   []byte{5, 5},
		},
	}
	stkDetails := ExtractMediaDetails(stickerMsg, "MSG_STK1", ts)
	if stkDetails.MediaType != "sticker" || !strings.HasSuffix(stkDetails.Filename, ".webp") {
		t.Errorf("Unexpected sticker details: %+v", stkDetails)
	}
}

func TestResolveLocalMediaPathSanitization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wame-path-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	localPath, _, err := ResolveLocalMediaPath(tmpDir, "12345@s.whatsapp.net", "MSG123", "../../etc/passwd")
	if err != nil {
		t.Fatalf("ResolveLocalMediaPath failed: %v", err)
	}
	if !strings.HasPrefix(localPath, tmpDir) || strings.Contains(localPath, "..") {
		t.Errorf("Path traversal not neutralized: %s", localPath)
	}
	if !strings.HasSuffix(localPath, "MSG123_passwd") {
		t.Errorf("Expected message-prefixed safe basename, got %s", localPath)
	}
}

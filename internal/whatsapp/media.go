package whatsapp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// MediaDownloader implements whatsmeow.DownloadableMessage
type MediaDownloader struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileLength    uint64
	FileSHA256    []byte
	FileEncSHA256 []byte
	MediaType     whatsmeow.MediaType
}

func (d *MediaDownloader) GetDirectPath() string {
	return d.DirectPath
}

func (d *MediaDownloader) GetURL() string {
	return d.URL
}

func (d *MediaDownloader) GetMediaKey() []byte {
	return d.MediaKey
}

func (d *MediaDownloader) GetFileLength() uint64 {
	return d.FileLength
}

func (d *MediaDownloader) GetFileSHA256() []byte {
	return d.FileSHA256
}

func (d *MediaDownloader) GetFileEncSHA256() []byte {
	return d.FileEncSHA256
}

func (d *MediaDownloader) GetMediaType() whatsmeow.MediaType {
	return d.MediaType
}

// ExtractDirectPathFromURL extracts direct path from a WhatsApp media URL
func ExtractDirectPathFromURL(url string) string {
	if url == "" {
		return ""
	}
	parts := strings.SplitN(url, ".net/", 2)
	if len(parts) < 2 {
		return url
	}
	pathPart := strings.SplitN(parts[1], "?", 2)[0]
	return "/" + pathPart
}

// UnwrapMessage unwraps nested WhatsApp wrapper messages (ephemeral, view-once,
// document-with-caption, device-sent, edited, etc.) to reach the inner payload.
func UnwrapMessage(msg *waProto.Message) *waProto.Message {
	curr := msg
	for depth := 0; curr != nil && depth < 8; depth++ {
		switch {
		case curr.GetEphemeralMessage().GetMessage() != nil:
			curr = curr.GetEphemeralMessage().GetMessage()
		case curr.GetViewOnceMessage().GetMessage() != nil:
			curr = curr.GetViewOnceMessage().GetMessage()
		case curr.GetViewOnceMessageV2().GetMessage() != nil:
			curr = curr.GetViewOnceMessageV2().GetMessage()
		case curr.GetViewOnceMessageV2Extension().GetMessage() != nil:
			curr = curr.GetViewOnceMessageV2Extension().GetMessage()
		case curr.GetDocumentWithCaptionMessage().GetMessage() != nil:
			curr = curr.GetDocumentWithCaptionMessage().GetMessage()
		case curr.GetDeviceSentMessage().GetMessage() != nil:
			curr = curr.GetDeviceSentMessage().GetMessage()
		case curr.GetEditedMessage().GetMessage() != nil:
			curr = curr.GetEditedMessage().GetMessage()
		case curr.GetGroupMentionedMessage().GetMessage() != nil:
			curr = curr.GetGroupMentionedMessage().GetMessage()
		case curr.GetBotInvokeMessage().GetMessage() != nil:
			curr = curr.GetBotInvokeMessage().GetMessage()
		case curr.GetProtocolMessage().GetEditedMessage() != nil:
			curr = curr.GetProtocolMessage().GetEditedMessage()
		default:
			return curr
		}
	}
	return curr
}

// ExtractTextContent extracts text or media caption from a protobuf Message
func ExtractTextContent(msg *waProto.Message) string {
	msg = UnwrapMessage(msg)
	if msg == nil {
		return ""
	}
	if text := msg.GetConversation(); text != "" {
		return text
	}
	if extended := msg.GetExtendedTextMessage(); extended != nil && extended.GetText() != "" {
		return extended.GetText()
	}
	if img := msg.GetImageMessage(); img != nil && img.GetCaption() != "" {
		return img.GetCaption()
	}
	if vid := msg.GetVideoMessage(); vid != nil && vid.GetCaption() != "" {
		return vid.GetCaption()
	}
	if ptv := msg.GetPtvMessage(); ptv != nil && ptv.GetCaption() != "" {
		return ptv.GetCaption()
	}
	if doc := msg.GetDocumentMessage(); doc != nil && doc.GetCaption() != "" {
		return doc.GetCaption()
	}
	return ""
}

// ExtractedMedia holds all metadata needed to index and download a WhatsApp attachment
type ExtractedMedia struct {
	MediaType     string
	Filename      string
	MimeType      string
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    uint64
}

func extensionForMime(mimeType, fallback string) string {
	base := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	switch base {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/3gpp":
		return ".3gp"
	case "audio/ogg", "application/ogg":
		return ".ogg"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/mp4", "audio/aac", "audio/x-m4a":
		return ".m4a"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "text/markdown":
		return ".md"
	case "application/json":
		return ".json"
	default:
		return fallback
	}
}

func sanitizeIDComponent(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	out := sb.String()
	if len(out) > 16 {
		out = out[len(out)-16:]
	}
	return out
}

// ExtractMediaDetails extracts full media metadata from a (possibly wrapped) protobuf Message
func ExtractMediaDetails(msg *waProto.Message, msgID string, ts time.Time) ExtractedMedia {
	msg = UnwrapMessage(msg)
	if msg == nil {
		return ExtractedMedia{}
	}

	if ts.IsZero() {
		ts = time.Now()
	}
	timestampStr := ts.Format("20060102_150405")
	idSuffix := ""
	if cleanID := sanitizeIDComponent(msgID); cleanID != "" {
		idSuffix = "_" + cleanID
	}

	if img := msg.GetImageMessage(); img != nil {
		mimeType := img.GetMimetype()
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		ext := extensionForMime(mimeType, ".jpg")
		return ExtractedMedia{
			MediaType:     "image",
			Filename:      fmt.Sprintf("image_%s%s%s", timestampStr, idSuffix, ext),
			MimeType:      mimeType,
			URL:           img.GetURL(),
			DirectPath:    img.GetDirectPath(),
			MediaKey:      img.GetMediaKey(),
			FileSHA256:    img.GetFileSHA256(),
			FileEncSHA256: img.GetFileEncSHA256(),
			FileLength:    img.GetFileLength(),
		}
	}

	if stk := msg.GetStickerMessage(); stk != nil {
		mimeType := stk.GetMimetype()
		if mimeType == "" {
			mimeType = "image/webp"
		}
		ext := extensionForMime(mimeType, ".webp")
		return ExtractedMedia{
			MediaType:     "sticker",
			Filename:      fmt.Sprintf("sticker_%s%s%s", timestampStr, idSuffix, ext),
			MimeType:      mimeType,
			URL:           stk.GetURL(),
			DirectPath:    stk.GetDirectPath(),
			MediaKey:      stk.GetMediaKey(),
			FileSHA256:    stk.GetFileSHA256(),
			FileEncSHA256: stk.GetFileEncSHA256(),
			FileLength:    stk.GetFileLength(),
		}
	}

	vid := msg.GetVideoMessage()
	if vid == nil {
		vid = msg.GetPtvMessage()
	}
	if vid != nil {
		mimeType := vid.GetMimetype()
		if mimeType == "" {
			mimeType = "video/mp4"
		}
		ext := extensionForMime(mimeType, ".mp4")
		return ExtractedMedia{
			MediaType:     "video",
			Filename:      fmt.Sprintf("video_%s%s%s", timestampStr, idSuffix, ext),
			MimeType:      mimeType,
			URL:           vid.GetURL(),
			DirectPath:    vid.GetDirectPath(),
			MediaKey:      vid.GetMediaKey(),
			FileSHA256:    vid.GetFileSHA256(),
			FileEncSHA256: vid.GetFileEncSHA256(),
			FileLength:    vid.GetFileLength(),
		}
	}

	if aud := msg.GetAudioMessage(); aud != nil {
		mimeType := aud.GetMimetype()
		if mimeType == "" {
			mimeType = "audio/ogg"
		}
		ext := extensionForMime(mimeType, ".ogg")
		return ExtractedMedia{
			MediaType:     "audio",
			Filename:      fmt.Sprintf("audio_%s%s%s", timestampStr, idSuffix, ext),
			MimeType:      mimeType,
			URL:           aud.GetURL(),
			DirectPath:    aud.GetDirectPath(),
			MediaKey:      aud.GetMediaKey(),
			FileSHA256:    aud.GetFileSHA256(),
			FileEncSHA256: aud.GetFileEncSHA256(),
			FileLength:    aud.GetFileLength(),
		}
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		mimeType := doc.GetMimetype()
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		fname := strings.TrimSpace(doc.GetFileName())
		if fname == "" {
			fname = strings.TrimSpace(doc.GetTitle())
		}
		if fname == "" {
			ext := extensionForMime(mimeType, "")
			fname = fmt.Sprintf("document_%s%s%s", timestampStr, idSuffix, ext)
		}
		return ExtractedMedia{
			MediaType:     "document",
			Filename:      fname,
			MimeType:      mimeType,
			URL:           doc.GetURL(),
			DirectPath:    doc.GetDirectPath(),
			MediaKey:      doc.GetMediaKey(),
			FileSHA256:    doc.GetFileSHA256(),
			FileEncSHA256: doc.GetFileEncSHA256(),
			FileLength:    doc.GetFileLength(),
		}
	}

	return ExtractedMedia{}
}

// ExtractMediaInfo extracts media metadata from a protobuf Message
func ExtractMediaInfo(msg *waProto.Message) (mediaType, filename, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) {
	m := ExtractMediaDetails(msg, "", time.Now())
	return m.MediaType, m.Filename, m.URL, m.MediaKey, m.FileSHA256, m.FileEncSHA256, m.FileLength
}

// AnalyzeOggOpus extracts duration and generates waveform for voice notes
func AnalyzeOggOpus(data []byte) (duration uint32, waveform []byte, err error) {
	if len(data) < 4 || string(data[0:4]) != "OggS" {
		return 0, nil, fmt.Errorf("not a valid Ogg file (missing OggS signature)")
	}

	var lastGranule uint64
	var sampleRate uint32 = 48000
	var preSkip uint16 = 0
	var foundOpusHead bool

	for i := 0; i < len(data); {
		if i+27 >= len(data) {
			break
		}
		if string(data[i:i+4]) != "OggS" {
			i++
			continue
		}

		granulePos := binary.LittleEndian.Uint64(data[i+6 : i+14])
		pageSeqNum := binary.LittleEndian.Uint32(data[i+18 : i+22])
		numSegments := int(data[i+26])

		if i+27+numSegments >= len(data) {
			break
		}
		segmentTable := data[i+27 : i+27+numSegments]

		pageSize := 27 + numSegments
		for _, segLen := range segmentTable {
			pageSize += int(segLen)
		}

		if !foundOpusHead && pageSeqNum <= 1 && i+pageSize <= len(data) {
			pageData := data[i : i+pageSize]
			headPos := bytes.Index(pageData, []byte("OpusHead"))
			if headPos >= 0 && headPos+16 < len(pageData) {
				headPos += 8
				preSkip = binary.LittleEndian.Uint16(pageData[headPos+10 : headPos+12])
				sampleRate = binary.LittleEndian.Uint32(pageData[headPos+12 : headPos+16])
				foundOpusHead = true
			}
		}

		if granulePos != 0 {
			lastGranule = granulePos
		}
		i += pageSize
	}

	if lastGranule > 0 && sampleRate > 0 {
		durationSeconds := float64(lastGranule-uint64(preSkip)) / float64(sampleRate)
		duration = uint32(math.Ceil(durationSeconds))
	} else {
		duration = uint32(float64(len(data)) / 2000.0)
	}

	if duration < 1 {
		duration = 1
	} else if duration > 300 {
		duration = 300
	}

	waveform = generatePlaceholderWaveform(duration)
	return duration, waveform, nil
}

func generatePlaceholderWaveform(duration uint32) []byte {
	const waveformLength = 64
	waveform := make([]byte, waveformLength)

	r := rand.New(rand.NewSource(int64(duration)))
	baseAmplitude := 35.0
	durFactor := math.Min(float64(duration), 120.0) / 30.0

	for i := range waveform {
		pos := float64(i) / float64(waveformLength)
		val := baseAmplitude*math.Sin(pos*math.Pi*durFactor*8) + (baseAmplitude/2)*math.Sin(pos*math.Pi*durFactor*16)
		val += (r.Float64() - 0.5) * 15
		fadeInOut := math.Sin(pos * math.Pi)
		val = val * (0.7 + 0.3*fadeInOut)
		val = val + 50

		if val < 0 {
			val = 0
		} else if val > 100 {
			val = 100
		}
		waveform[i] = byte(val)
	}
	return waveform
}

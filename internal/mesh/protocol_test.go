package mesh

import (
	"reflect"
	"testing"
	"time"
)

func TestBitchatMessage_EncodeDecode(t *testing.T) {
	originalMsg := &BitchatMessage{
		ID:                "msg-123",
		Sender:            "user-a",
		Content:           "Olá, mundo!",
		Timestamp:         time.Now().UTC().Truncate(time.Millisecond), // Truncate for consistent comparison
		IsRelay:           true,
		IsPrivate:         true,
		IsEncrypted:       true,
		OriginalSender:    "user-b",
		RecipientNickname: "user-c",
		SenderPeerID:      "peer-id-123",
		Channel:           "#geral",
		Mentions:          []string{"user-d", "user-e"},
		EncryptedContent:  []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	t.Run("Test Full Message Roundtrip", func(t *testing.T) {
		// Encode
		encodedData, err := originalMsg.Encode()
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}

		// Decode
		decodedMsg, err := DecodeMessage(encodedData)
		if err != nil {
			t.Fatalf("DecodeMessage() error = %v", err)
		}

		// Compare
		if !reflect.DeepEqual(originalMsg, decodedMsg) {
			t.Errorf("Decoded message does not match original.\nOriginal: %+v\nDecoded:  %+v", originalMsg, decodedMsg)
		}
	})

	t.Run("Test Minimal Message Roundtrip", func(t *testing.T) {
		minimalMsg := &BitchatMessage{
			ID:        "min-msg-456",
			Sender:    "user-min",
			Content:   "Mínimo.",
			Timestamp: time.Now().UTC().Truncate(time.Millisecond),
		}

		// Encode
		encodedData, err := minimalMsg.Encode()
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}

		// Decode
		decodedMsg, err := DecodeMessage(encodedData)
		if err != nil {
			t.Fatalf("DecodeMessage() error = %v", err)
		}

		// Compare
		if !reflect.DeepEqual(minimalMsg, decodedMsg) {
			t.Errorf("Decoded minimal message does not match original.\nOriginal: %+v\nDecoded:  %+v", minimalMsg, decodedMsg)
		}
	})
}

package mesh

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// MessageType defines the type of a BitchatPacket.
type MessageType byte

const (
	Announce                MessageType = 0x01
	Leave                   MessageType = 0x03
	Message                 MessageType = 0x04
	FragmentStart           MessageType = 0x05
	FragmentContinue        MessageType = 0x06
	FragmentEnd             MessageType = 0x07
	ChannelAnnounce         MessageType = 0x08
	DeliveryAck             MessageType = 0x0A
	DeliveryStatusRequest   MessageType = 0x0B
	ReadReceipt             MessageType = 0x0C
	NoiseHandshakeInit      MessageType = 0x10
	NoiseHandshakeResp      MessageType = 0x11
	NoiseEncrypted          MessageType = 0x12
	NoiseIdentityAnnounce   MessageType = 0x13
	ChannelKeyVerifyRequest MessageType = 0x14
	ChannelKeyVerifyResponse MessageType = 0x15
	ChannelPasswordUpdate   MessageType = 0x16
	ChannelMetadata         MessageType = 0x17
	VersionHello            MessageType = 0x20
	VersionAck              MessageType = 0x21
)

// BitchatPacket represents the outer layer of communication.
type BitchatPacket struct {
	Version     byte
	Type        MessageType
	TTL         byte
	Timestamp   uint64 // Milliseconds
	Flags       byte
	Payload     []byte
	SenderID    []byte // 8 bytes
	RecipientID []byte // 8 bytes, optional
	Signature   []byte // 64 bytes, optional
}

// BitchatMessage represents the inner user-level message.
type BitchatMessage struct {
	ID                string
	Sender            string
	Content           string
	Timestamp         time.Time
	IsRelay           bool
	IsPrivate         bool
	IsEncrypted       bool
	OriginalSender    string   // Optional
	RecipientNickname string   // Optional
	SenderPeerID      string   // Optional
	Mentions          []string // Optional
	Channel           string   // Optional
	EncryptedContent  []byte   // Optional
}

// Packet flags
const (
	FlagHasRecipient byte = 0x01
	FlagHasSignature byte = 0x02
	FlagIsCompressed byte = 0x04
)

const (
	HeaderSize      = 14 // 1 (Version) + 1 (Type) + 1 (TTL) + 8 (Timestamp) + 1 (Flags) + 2 (PayloadLength)
	SenderIDSize    = 8
	RecipientIDSize = 8
	SignatureSize   = 64
)

// EncodePacket serializes a BitchatPacket into a byte slice.
func (p *BitchatPacket) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Header (13 bytes)
	header := make([]byte, HeaderSize)
	header[0] = p.Version
	header[1] = byte(p.Type)
	header[2] = p.TTL
	binary.BigEndian.PutUint64(header[3:11], p.Timestamp)

	var flags byte
	if len(p.RecipientID) > 0 {
		flags |= FlagHasRecipient
	}
	if len(p.Signature) > 0 {
		flags |= FlagHasSignature
	}
	// TODO: Add compression flag if implemented
	header[11] = flags

	binary.BigEndian.PutUint16(header[12:14], uint16(len(p.Payload)))

	buf.Write(header)

	// Variable sections
	buf.Write(p.SenderID)
	if (flags & FlagHasRecipient) != 0 {
		buf.Write(p.RecipientID)
	}
	buf.Write(p.Payload)
	if (flags & FlagHasSignature) != 0 {
		buf.Write(p.Signature)
	}

	return buf.Bytes(), nil
}

// DecodePacket deserializes a byte slice into a BitchatPacket.
// Message flags for optional fields
const (
	MsgFlagIsRelay           byte = 0x01
	MsgFlagIsPrivate         byte = 0x02
	MsgFlagIsEncrypted       byte = 0x04
	MsgFlagHasOriginalSender byte = 0x08
	MsgFlagHasRecipientNick  byte = 0x10
	MsgFlagHasSenderPeerID   byte = 0x20
	MsgFlagHasMentions       byte = 0x40
	MsgFlagHasChannel        byte = 0x80
)

// EncodeMessage serializes a BitchatMessage into a byte slice.
func (m *BitchatMessage) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)

	// 1. Write mandatory fields
	// ID, Sender, Content
	writeString(buf, m.ID)
	writeString(buf, m.Sender)
	writeString(buf, m.Content)

	// Timestamp
	binary.Write(buf, binary.BigEndian, m.Timestamp.UnixMilli())

	// 2. Determine and write flags
	var flags byte
	if m.IsRelay { flags |= MsgFlagIsRelay }
	if m.IsPrivate { flags |= MsgFlagIsPrivate }
	if m.IsEncrypted { flags |= MsgFlagIsEncrypted }
	if m.OriginalSender != "" { flags |= MsgFlagHasOriginalSender }
	if m.RecipientNickname != "" { flags |= MsgFlagHasRecipientNick }
	if m.SenderPeerID != "" { flags |= MsgFlagHasSenderPeerID }
	if len(m.Mentions) > 0 { flags |= MsgFlagHasMentions }
	if m.Channel != "" { flags |= MsgFlagHasChannel }
	buf.WriteByte(flags)

	// 3. Write optional fields
	if (flags&MsgFlagHasOriginalSender) != 0 { writeString(buf, m.OriginalSender) }
	if (flags&MsgFlagHasRecipientNick) != 0 { writeString(buf, m.RecipientNickname) }
	if (flags&MsgFlagHasSenderPeerID) != 0 { writeString(buf, m.SenderPeerID) }
	if (flags&MsgFlagHasChannel) != 0 { writeString(buf, m.Channel) }

	if (flags&MsgFlagHasMentions) != 0 {
		buf.WriteByte(byte(len(m.Mentions)))
		for _, mention := range m.Mentions {
			writeString(buf, mention)
		}
	}

	if m.IsEncrypted {
		binary.Write(buf, binary.BigEndian, uint32(len(m.EncryptedContent)))
		buf.Write(m.EncryptedContent)
	}

	return buf.Bytes(), nil
}

// DecodeMessage deserializes a byte slice into a BitchatMessage.
func DecodeMessage(data []byte) (*BitchatMessage, error) {
	m := &BitchatMessage{}
	buf := bytes.NewReader(data)
	var err error

	// 1. Read mandatory fields
	m.ID, err = readString(buf); if err != nil { return nil, err }
	m.Sender, err = readString(buf); if err != nil { return nil, err }
	m.Content, err = readString(buf); if err != nil { return nil, err }

	var ts int64
	err = binary.Read(buf, binary.BigEndian, &ts)
	if err != nil { return nil, errors.New("could not read timestamp") }
	m.Timestamp = time.UnixMilli(ts).UTC()

	// 2. Read flags
	flags, err := buf.ReadByte()
	if err != nil { return nil, errors.New("could not read flags") }

	m.IsRelay = (flags & MsgFlagIsRelay) != 0
	m.IsPrivate = (flags & MsgFlagIsPrivate) != 0
	m.IsEncrypted = (flags & MsgFlagIsEncrypted) != 0

	// 3. Read optional fields
	if (flags&MsgFlagHasOriginalSender) != 0 { m.OriginalSender, err = readString(buf); if err != nil { return nil, err } }
	if (flags&MsgFlagHasRecipientNick) != 0 { m.RecipientNickname, err = readString(buf); if err != nil { return nil, err } }
	if (flags&MsgFlagHasSenderPeerID) != 0 { m.SenderPeerID, err = readString(buf); if err != nil { return nil, err } }
	if (flags&MsgFlagHasChannel) != 0 { m.Channel, err = readString(buf); if err != nil { return nil, err } }

	if (flags&MsgFlagHasMentions) != 0 {
		mentionCount, err := buf.ReadByte()
		if err != nil { return nil, errors.New("could not read mention count") }
		m.Mentions = make([]string, mentionCount)
		for i := 0; i < int(mentionCount); i++ {
			m.Mentions[i], err = readString(buf); if err != nil { return nil, err }
		}
	}

	if m.IsEncrypted {
		var contentLength uint32
		err = binary.Read(buf, binary.BigEndian, &contentLength)
		if err != nil { return nil, errors.New("could not read encrypted content length") }
		m.EncryptedContent = make([]byte, contentLength)
		_, err = buf.Read(m.EncryptedContent); if err != nil { return nil, err }
	}

	return m, nil
}

// writeString is a helper to write a length-prefixed string.
func writeString(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.BigEndian, uint16(len(s)))
	buf.WriteString(s)
}

// readString is a helper to read a length-prefixed string.
func readString(buf *bytes.Reader) (string, error) {
	var length uint16
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return "", errors.New("could not read string length")
	}
	strBytes := make([]byte, length)
	if _, err := buf.Read(strBytes); err != nil {
		return "", errors.New("could not read string data")
	}
	return string(strBytes), nil
}

func DecodePacket(data []byte) (*BitchatPacket, error) {
	if len(data) < HeaderSize+SenderIDSize {
		return nil, errors.New("packet too small")
	}

	p := &BitchatPacket{}
	buf := bytes.NewReader(data)

	// Header
	header := make([]byte, HeaderSize)
	_, err := buf.Read(header)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	p.Version = header[0]
	p.Type = MessageType(header[1])
	p.TTL = header[2]
	p.Timestamp = binary.BigEndian.Uint64(header[3:11])
	p.Flags = header[11]
	payloadLength := binary.BigEndian.Uint16(header[12:14])

	// Sender ID
	p.SenderID = make([]byte, SenderIDSize)
	_, err = buf.Read(p.SenderID)
	if err != nil {
		return nil, fmt.Errorf("failed to read sender id: %w", err)
	}

	// Recipient ID (optional)
	if (p.Flags & FlagHasRecipient) != 0 {
		p.RecipientID = make([]byte, RecipientIDSize)
		_, err = buf.Read(p.RecipientID)
		if err != nil {
			return nil, fmt.Errorf("failed to read recipient id: %w", err)
		}
	}

	// Payload (only read if length > 0)
	if payloadLength > 0 {
		p.Payload = make([]byte, payloadLength)
		_, err = buf.Read(p.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	// Signature (optional)
	if (p.Flags & FlagHasSignature) != 0 {
		p.Signature = make([]byte, SignatureSize)
		_, err = buf.Read(p.Signature)
		if err != nil {
			return nil, fmt.Errorf("failed to read signature: %w", err)
		}
	}

	return p, nil
}

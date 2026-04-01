package bedrock

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

// AWS Event Stream binary message format:
// [total_length:4][headers_length:4][prelude_crc:4][headers:*][payload:*][message_crc:4]
//
// Header format:
// [name_length:1][name:*][type:1][value_length:2][value:*]
//
// Header value types: 0=bool_true, 1=bool_false, 2=byte, 3=short, 4=int, 5=long,
//                     6=bytes, 7=string, 8=timestamp, 9=uuid

// eventStreamMessage represents a decoded AWS event stream message.
type eventStreamMessage struct {
	Headers map[string]string
	Payload []byte
}

// decodeEventStreamMessage reads a single AWS event stream message from the reader.
// Returns the decoded message or an error (io.EOF when stream ends).
func decodeEventStreamMessage(r io.Reader) (*eventStreamMessage, error) {
	// Read prelude: total_length (4) + headers_length (4) + prelude_crc (4)
	prelude := make([]byte, 12)
	if _, err := io.ReadFull(r, prelude); err != nil {
		return nil, err
	}

	totalLength := binary.BigEndian.Uint32(prelude[0:4])
	headersLength := binary.BigEndian.Uint32(prelude[4:8])
	preludeCRC := binary.BigEndian.Uint32(prelude[8:12])

	// Verify prelude CRC
	computedPreludeCRC := crc32.ChecksumIEEE(prelude[0:8])
	if computedPreludeCRC != preludeCRC {
		return nil, fmt.Errorf("prelude CRC mismatch: got %d, want %d", computedPreludeCRC, preludeCRC)
	}

	// Remaining bytes = total - prelude(12) - message_crc(4)
	remaining := int(totalLength) - 12 - 4
	if remaining < 0 {
		return nil, fmt.Errorf("invalid total length: %d", totalLength)
	}

	body := make([]byte, remaining)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("reading message body: %w", err)
	}

	// Read message CRC
	crcBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, crcBuf); err != nil {
		return nil, fmt.Errorf("reading message CRC: %w", err)
	}
	messageCRC := binary.BigEndian.Uint32(crcBuf)

	// Verify message CRC (over prelude + body)
	crcData := append(prelude, body...)
	computedMessageCRC := crc32.ChecksumIEEE(crcData)
	if computedMessageCRC != messageCRC {
		return nil, fmt.Errorf("message CRC mismatch: got %d, want %d", computedMessageCRC, messageCRC)
	}

	// Parse headers
	headers := make(map[string]string)
	headerBytes := body[:headersLength]
	offset := 0
	for offset < len(headerBytes) {
		if offset >= len(headerBytes) {
			break
		}

		// Name length (1 byte)
		nameLen := int(headerBytes[offset])
		offset++

		if offset+nameLen > len(headerBytes) {
			break
		}
		name := string(headerBytes[offset : offset+nameLen])
		offset += nameLen

		if offset >= len(headerBytes) {
			break
		}

		// Value type (1 byte)
		valueType := headerBytes[offset]
		offset++

		switch valueType {
		case 7: // string
			if offset+2 > len(headerBytes) {
				break
			}
			valueLen := int(binary.BigEndian.Uint16(headerBytes[offset : offset+2]))
			offset += 2
			if offset+valueLen > len(headerBytes) {
				break
			}
			headers[name] = string(headerBytes[offset : offset+valueLen])
			offset += valueLen
		case 6: // bytes
			if offset+2 > len(headerBytes) {
				break
			}
			valueLen := int(binary.BigEndian.Uint16(headerBytes[offset : offset+2]))
			offset += 2
			offset += valueLen
		case 0, 1: // bool_true, bool_false
			// no value bytes
		case 2: // byte
			offset++
		case 3: // short
			offset += 2
		case 4: // int
			offset += 4
		case 5: // long
			offset += 8
		case 8: // timestamp
			offset += 8
		case 9: // uuid
			offset += 16
		default:
			return nil, fmt.Errorf("unknown header value type: %d", valueType)
		}
	}

	// Extract payload (after headers)
	payload := body[headersLength:]

	return &eventStreamMessage{
		Headers: headers,
		Payload: payload,
	}, nil
}

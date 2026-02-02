package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// Base64PCMToWAV converts base64-encoded PCM audio into a WAV byte slice.
func Base64PCMToWAV(
	base64PCM string,
	sampleRate int,
	channels int,
	bitsPerSample int,
) ([]byte, error) {

	// Decode base64 PCM
	pcmData, err := base64.StdEncoding.DecodeString(base64PCM)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(pcmData)

	buf := &bytes.Buffer{}

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16)) // Subchunk1Size
	binary.Write(buf, binary.LittleEndian, uint16(1))  // PCM format
	binary.Write(buf, binary.LittleEndian, uint16(channels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcmData)

	return buf.Bytes(), nil
}

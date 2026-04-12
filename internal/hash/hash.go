package hash

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// CanonicalJSONBytes is the single JSON serialization contract shared by
// WriteJSON and SHA256HexJSON.
func CanonicalJSONBytes(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("encode canonical json: %w", err)
	}

	payload := buffer.Bytes()
	if len(payload) > 0 && payload[len(payload)-1] == '\n' {
		payload = payload[:len(payload)-1]
	}

	return append([]byte(nil), payload...), nil
}

// SHA256Hex returns the lowercase SHA-256 digest for the provided bytes.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SHA256HexFile returns the lowercase SHA-256 digest for an on-disk file.
func SHA256HexFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for hashing: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash file contents: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// SHA256HexJSON hashes the exact canonical JSON byte stream emitted by
// CanonicalJSONBytes and therefore by storage.WriteJSON.
func SHA256HexJSON(value any) (string, error) {
	payload, err := CanonicalJSONBytes(value)
	if err != nil {
		return "", err
	}
	return SHA256Hex(payload), nil
}

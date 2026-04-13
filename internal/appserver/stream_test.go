package appserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStreamDecoderDecodesFramedInitialize(t *testing.T) {
	decoder := newTestStreamDecoder(t, "framed_initialize.txt")
	assertInitializeEnvelope(t, decoder)
}

func TestNewStreamDecoderFallsBackToSequentialJSON(t *testing.T) {
	decoder := newTestStreamDecoder(t, "sequential_initialize.json")
	assertInitializeEnvelope(t, decoder)
}

func TestNewStreamDecoderDecodesRepeatedFramedMessages(t *testing.T) {
	frame := readTestData(t, "framed_initialize.txt")
	decoder := newTestStreamDecoderFromBytes(t, append(bytes.Clone(frame), frame...))

	assertInitializeEnvelope(t, decoder)
	assertInitializeEnvelope(t, decoder)
}

func TestNewStreamDecoderRejectsExtraJSONInSingleFrame(t *testing.T) {
	payload := "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}{\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{}}"
	decoder := newTestStreamDecoderFromBytes(t, framedMessage(payload))

	var envelope rawEnvelope
	err := decoder.Decode(&envelope)
	if err == nil {
		t.Fatal("Decode returned nil error, want malformed frame error")
	}
	if !errors.Is(err, errMalformedFramePayload) {
		t.Fatalf("Decode error = %v, want malformed frame payload error", err)
	}
	if errors.Is(err, io.EOF) {
		t.Fatalf("Decode error = %v, want malformed frame error", err)
	}
}

func TestNewStreamDecoderRejectsEmptyOrWhitespaceOnlyFramePayload(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assertMalformedFramePayload(t, framedMessage(""))
	})

	t.Run("whitespace", func(t *testing.T) {
		assertMalformedFramePayload(t, framedMessage(" \r\n\t "))
	})
}

func assertMalformedFramePayload(t *testing.T, data []byte) {
	t.Helper()

	decoder := newTestStreamDecoderFromBytes(t, data)

	var envelope rawEnvelope
	err := decoder.Decode(&envelope)
	if err == nil {
		t.Fatal("Decode returned nil error, want malformed frame error")
	}
	if !errors.Is(err, errMalformedFramePayload) {
		t.Fatalf("Decode error = %v, want malformed frame payload error", err)
	}
	if errors.Is(err, io.EOF) {
		t.Fatalf("Decode error = %v, want malformed frame error", err)
	}
}

func newTestStreamDecoder(t *testing.T, name string) StreamDecoder {
	t.Helper()

	return newTestStreamDecoderFromBytes(t, readTestData(t, name))
}

func newTestStreamDecoderFromBytes(t *testing.T, data []byte) StreamDecoder {
	t.Helper()

	decoder, err := NewStreamDecoder(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("NewStreamDecoder returned error: %v", err)
	}

	return decoder
}

func readTestData(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func framedMessage(payload string) []byte {
	return []byte(fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload))
}

func assertInitializeEnvelope(t *testing.T, decoder StreamDecoder) {
	t.Helper()

	var envelope rawEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	if envelope.JSONRPC != JSONRPCVersion {
		t.Fatalf("jsonrpc = %q, want %q", envelope.JSONRPC, JSONRPCVersion)
	}

	if string(envelope.ID) != "1" {
		t.Fatalf("id = %s, want 1", envelope.ID)
	}

	var result InitializeResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal(result) returned error: %v", err)
	}

	if result.ProtocolVersion != "2026-04-13" {
		t.Fatalf("protocolVersion = %q, want %q", result.ProtocolVersion, "2026-04-13")
	}

	if result.ServerInfo == nil {
		t.Fatal("serverInfo is nil")
	}

	if result.ServerInfo.Name != "codex-cli" {
		t.Fatalf("serverInfo.name = %q, want %q", result.ServerInfo.Name, "codex-cli")
	}

	if result.ServerInfo.Version != "0.0.0" {
		t.Fatalf("serverInfo.version = %q, want %q", result.ServerInfo.Version, "0.0.0")
	}

	if _, ok := result.Capabilities["items"]; !ok {
		t.Fatalf("capabilities missing %q key", "items")
	}
}

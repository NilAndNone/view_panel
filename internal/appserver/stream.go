package appserver

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const framedContentLengthPrefix = "content-length:"

var errMalformedFramePayload = errors.New("malformed frame payload")

type StreamDecoder interface {
	Decode(v any) error
}

type streamDecoder struct {
	sequential *json.Decoder
	framed     *framedDecoder
}

func NewStreamDecoder(r io.Reader) (StreamDecoder, error) {
	reader := bufio.NewReader(r)

	prefix, framed, err := probeStreamPrefix(reader)
	if err != nil {
		return nil, err
	}

	rewound := io.MultiReader(bytes.NewReader(prefix), reader)
	if framed {
		return &streamDecoder{framed: newFramedDecoder(rewound)}, nil
	}

	decoder := json.NewDecoder(rewound)
	decoder.UseNumber()

	return &streamDecoder{sequential: decoder}, nil
}

func (d *streamDecoder) Decode(v any) error {
	if d.framed != nil {
		return d.framed.Decode(v)
	}
	return d.sequential.Decode(v)
}

func probeStreamPrefix(reader *bufio.Reader) ([]byte, bool, error) {
	prefix := make([]byte, 0, len(framedContentLengthPrefix))

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return prefix, false, nil
			}
			return nil, false, err
		}

		prefix = append(prefix, b)
		if isTransportWhitespace(b) {
			continue
		}
		if lowerASCIILetter(b) != framedContentLengthPrefix[0] {
			return prefix, false, nil
		}
		break
	}

	for i := 1; i < len(framedContentLengthPrefix); i++ {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return prefix, false, nil
			}
			return nil, false, err
		}

		prefix = append(prefix, b)
		if lowerASCIILetter(b) != framedContentLengthPrefix[i] {
			return prefix, false, nil
		}
	}

	return prefix, true, nil
}

func isTransportWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func lowerASCIILetter(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

type framedDecoder struct {
	reader *bufio.Reader
}

func newFramedDecoder(r io.Reader) *framedDecoder {
	return &framedDecoder{reader: bufio.NewReader(r)}
}

func (d *framedDecoder) Decode(v any) error {
	payload, err := d.readFrame()
	if err != nil {
		return err
	}

	if len(bytes.TrimSpace(payload)) == 0 {
		return fmt.Errorf("%w: empty frame body", errMalformedFramePayload)
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("%w: decode frame body: %v", errMalformedFramePayload, err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("%w: trailing data after first JSON value: %v", errMalformedFramePayload, err)
	}

	return fmt.Errorf("%w: frame body contains multiple JSON values", errMalformedFramePayload)
}

func (d *framedDecoder) readFrame() ([]byte, error) {
	if err := discardLeadingTransportWhitespace(d.reader); err != nil {
		return nil, err
	}

	contentLength := -1
	for {
		line, err := d.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			return nil, fmt.Errorf("invalid frame header %q", line)
		}

		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid content length %q: %w", value, err)
			}
			if parsed < 0 {
				return nil, fmt.Errorf("invalid content length %d", parsed)
			}
			contentLength = parsed
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(d.reader, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func discardLeadingTransportWhitespace(reader *bufio.Reader) error {
	for {
		peek, err := reader.Peek(1)
		if err != nil {
			return err
		}
		if !isTransportWhitespace(peek[0]) {
			return nil
		}
		if _, err := reader.ReadByte(); err != nil {
			return err
		}
	}
}

package service

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// DecodeDataURL parses a base64 data URL of the form "data:<mime>;base64,<payload>".
func DecodeDataURL(dataURL string) (mime string, data []byte, err error) {
	const prefix = "data:"
	if !strings.HasPrefix(dataURL, prefix) {
		return "", nil, fmt.Errorf("invalid data URL: missing data: prefix")
	}

	payloadIndex := strings.Index(dataURL, ";base64,")
	if payloadIndex == -1 {
		return "", nil, fmt.Errorf("invalid data URL: missing base64 payload")
	}

	mimePart := dataURL[len(prefix):payloadIndex]
	if mimePart == "" {
		return "", nil, fmt.Errorf("invalid data URL: missing mime type")
	}
	if strings.Contains(mimePart, ";") {
		return "", nil, fmt.Errorf("invalid data URL: unexpected parameters")
	}

	payload := dataURL[payloadIndex+len(";base64,"):]
	decoded, decodeErr := base64.StdEncoding.DecodeString(payload)
	if decodeErr != nil {
		return "", nil, fmt.Errorf("invalid data URL: base64 decode failed: %w", decodeErr)
	}

	return mimePart, decoded, nil
}

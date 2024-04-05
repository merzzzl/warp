package wireguard

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

var errKeyInvalid = errors.New("invalid wireguard key")

func encodeBase64ToHex(key string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("invalid base64 string (%s): %w", key, err)
	}

	if len(decoded) != 32 {
		return "", fmt.Errorf("key should be 32 bytes (%s): %w", key, errKeyInvalid)
	}

	return hex.EncodeToString(decoded), nil
}

package modules

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const (
	AccessPrefix  = "ua_"
	RefreshPrefix = "ur_"
	GamePrefix    = "mc_"
)

func GenerateToken(prefix string) string {
	var random [32]byte
	rand.Read(random[:])
	return prefix + base64.RawURLEncoding.EncodeToString(random[:])
}

func TokenDigest(prefix, token string) (string, bool) {
	raw, ok := strings.CutPrefix(token, prefix)
	if !ok || len(raw) != base64.RawURLEncoding.EncodedLen(32) {
		return "", false
	}
	if _, err := base64.RawURLEncoding.Strict().DecodeString(raw); err != nil {
		return "", false
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), true
}

func ClientToken() string {
	var random [16]byte
	rand.Read(random[:])
	return base64.RawURLEncoding.EncodeToString(random[:])
}

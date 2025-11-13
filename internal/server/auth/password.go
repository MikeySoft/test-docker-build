package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

// HashPassword hashes a plaintext password using argon2id and returns a PHC formatted string
func HashPassword(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt gen: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)
	// PHC string
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, b64Salt, b64Key), nil
}

// VerifyPassword compares a plaintext password with a PHC formatted argon2id hash
func VerifyPassword(plain, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("unsupported hash format")
	}
	// parts[2]=v=19, parts[3]=m=...,t=...,p=..., parts[4]=salt, parts[5]=hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("bad salt: %w", err)
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("bad hash: %w", err)
	}
	keyLen := len(expected)
	if keyLen == 0 {
		return false, errors.New("invalid hash length")
	}
	var outLen uint32
	if keyLen > math.MaxUint32 {
		outLen = math.MaxUint32
	} else {
		outLen = uint32(keyLen)
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, outLen)
	if subtle.ConstantTimeCompare(key, expected) == 1 {
		return true, nil
	}
	return false, nil
}

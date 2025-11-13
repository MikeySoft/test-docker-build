package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

var (
	rsaOnce sync.Once
	rsaPriv *rsa.PrivateKey
	rsaPub  *rsa.PublicKey
)

func initKeys() {
	rsaOnce.Do(func() {
		// Try to load existing keys from environment variables or files
		var privData []byte

		if envPriv := strings.TrimSpace(os.Getenv("JWT_PRIVATE_KEY")); envPriv != "" {
			privData = []byte(envPriv)
		} else {
			privFile := strings.TrimSpace(os.Getenv("JWT_PRIVATE_KEY_FILE"))
			if privFile == "" {
				privFile = "jwt_private.pem"
			}
			// #nosec G304 -- path comes from trusted admin configuration
			if data, err := os.ReadFile(privFile); err == nil {
				privData = data
			}
		}

		if len(privData) > 0 {
			if key, err := jwt.ParseRSAPrivateKeyFromPEM(privData); err == nil {
				rsaPriv = key
				rsaPub = &key.PublicKey
				return
			}
		}

		// Generate new keys if loading failed
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return
		}
		rsaPriv = key
		rsaPub = &key.PublicKey
	})
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func SignAccessToken(subject, username, role, jti string, ttl time.Duration) (string, error) {
	initKeys()
	now := time.Now()
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(rsaPriv)
}

func ParseAccessToken(tokenStr string) (*Claims, error) {
	initKeys()
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return rsaPub, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// JWKS minimal response for RS256 single key (kid omitted for brevity)
func ServeJWKS(w http.ResponseWriter, r *http.Request) {
	initKeys()
	// For MVP we do not expose full JWKS; placeholder returning empty set
	_ = r
	_ = rsaPub
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
}

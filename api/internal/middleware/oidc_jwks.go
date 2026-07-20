package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"fly-print-cloud/api/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

const jwksCacheTTL = 5 * time.Minute

type oidcJWK struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type oidcJWKS struct {
	Keys []oidcJWK `json:"keys"`
}

var oidcKeyCache struct {
	sync.Mutex
	url       string
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

func parseOIDCToken(tokenString string, cfg *config.OAuth2Config) (*OAuth2TokenInfo, error) {
	if cfg == nil || cfg.JWKSURL == "" || cfg.JWTIssuer == "" || cfg.Audience == "" {
		return nil, fmt.Errorf("OIDC JWT validation is not configured")
	}
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg(), jwt.SigningMethodRS384.Alg(), jwt.SigningMethodRS512.Alg()}),
		jwt.WithIssuer(cfg.JWTIssuer),
		jwt.WithAudience(cfg.Audience),
		jwt.WithExpirationRequired(),
	)
	token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("OIDC token has no kid")
		}
		return getOIDCKey(cfg.JWKSURL, kid)
	})
	if err != nil || !token.Valid {
		if err == nil {
			err = fmt.Errorf("invalid OIDC JWT")
		}
		return nil, err
	}
	parsedClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid OIDC claims")
	}
	return tokenInfoFromClaims(parsedClaims), nil
}

func getOIDCKey(jwksURL, kid string) (*rsa.PublicKey, error) {
	oidcKeyCache.Lock()
	defer oidcKeyCache.Unlock()
	if oidcKeyCache.url != jwksURL || time.Now().After(oidcKeyCache.expiresAt) {
		if err := refreshOIDCKeysLocked(jwksURL); err != nil {
			return nil, err
		}
	}
	key := oidcKeyCache.keys[kid]
	if key == nil {
		// Key rotation may introduce a new kid before the cache expires.
		if err := refreshOIDCKeysLocked(jwksURL); err != nil {
			return nil, err
		}
		key = oidcKeyCache.keys[kid]
	}
	if key == nil {
		return nil, fmt.Errorf("OIDC signing key %q not found", kid)
	}
	return key, nil
}

func refreshOIDCKeysLocked(jwksURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(jwksURL)
	if err != nil {
		return fmt.Errorf("fetch OIDC JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch OIDC JWKS: unexpected status %d", resp.StatusCode)
	}
	var document oidcJWKS
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&document); err != nil {
		return fmt.Errorf("decode OIDC JWKS: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, jwk := range document.Keys {
		if jwk.KTY != "RSA" || jwk.KID == "" {
			continue
		}
		n, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}
		e, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		exponent := 0
		for _, b := range e { exponent = exponent<<8 | int(b) }
		if exponent < 3 {
			continue
		}
		keys[jwk.KID] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
	}
	if len(keys) == 0 {
		return fmt.Errorf("OIDC JWKS contains no usable RSA keys")
	}
	oidcKeyCache.url = jwksURL
	oidcKeyCache.keys = keys
	oidcKeyCache.expiresAt = time.Now().Add(jwksCacheTTL)
	return nil
}

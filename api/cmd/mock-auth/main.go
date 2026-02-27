package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// simple in-process mock auth server for local testing
// it issues JWT access tokens whose claims match the expectations
// of the OAuth2 middleware in this project.

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int64  `json:"expires_in"`
}

func main() {
	addr := flag.String("addr", ":9090", "listen address, e.g. :9090")
	flag.Parse()

	http.HandleFunc("/test/token", handleTestToken)
	http.HandleFunc("/auth", handleAuth)
	http.HandleFunc("/token", handleToken)
	http.HandleFunc("/userinfo", handleUserInfo)

	log.Printf("Mock Auth server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// handleTestToken issues a JWT access token for testing.
//
// Query parameters:
//   type:  edge | admin | custom  (default: edge)
//   scopes: space separated scopes, overrides defaults when type=custom
//   sub:    subject (defaults depend on type)
var authCodes = map[string]string{}

func generateToken(sub, username, email, scope string) (string, error) {
	claims := jwt.MapClaims{
		"sub":                sub,
		"preferred_username": username,
		"email":              email,
		"scope":              scope,
		"realm_access": map[string]interface{}{
			"roles": strings.Fields(scope),
		},
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte("mock-auth-secret"))
}

func handleTestToken(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	if typ == "" {
		typ = "edge"
	}

	var (
		sub      string
		username string
		email    string
		scope    string
	)

	switch typ {
	case "edge":
		sub = "edge-node-demo"
		username = "edge-node-demo"
		email = "edge-node-demo@example.com"
		scope = "edge:register edge:printer edge:heartbeat file:read"
	case "admin":
		sub = "admin-user"
		username = "admin"
		email = "admin@example.com"
		scope = "admin edge:register edge:printer edge:heartbeat file:read"
	case "custom":
		sub = q.Get("sub")
		if sub == "" {
			sub = "custom-user"
		}
		username = sub
		email = sub + "@example.com"
		scope = q.Get("scopes")
		if scope == "" {
			scope = "edge:register edge:printer"
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "unknown type: %s", typ)
		return
	}

	if qs := q.Get("scopes"); qs != "" {
		scope = qs
	}
	if qs := q.Get("sub"); qs != "" {
		sub = qs
	}

	signed, err := generateToken(sub, username, email, scope)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to sign token: %v", err)
		return
	}

	resp := tokenResponse{
		AccessToken: signed,
		TokenType:   "bearer",
		Scope:       scope,
		ExpiresIn:   3600,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// /auth: OAuth2 授权端点，直接重定向回 redirect_uri，附带 code 和原始 state
func handleAuth(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "missing redirect_uri")
		return
	}
	state := q.Get("state")

	code := fmt.Sprintf("code-%d", time.Now().UnixNano())
	authCodes[code] = "admin-user"

	u, err := url.Parse(redirectURI)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid redirect_uri: %v", err)
		return
	}
	vals := u.Query()
	vals.Set("code", code)
	if state != "" {
		vals.Set("state", state)
	}
	u.RawQuery = vals.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

// /token: OAuth2 token 端点，根据授权码返回访问令牌
func handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid form: %v", err)
		return
	}

	code := r.Form.Get("code")
	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "missing code")
		return
	}

	// 简化：忽略 client_id / client_secret 校验，只要 code 存在即认为合法
	if _, ok := authCodes[code]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "invalid code")
		return
	}

	sub := "admin-user"
	username := "admin"
	email := "admin@example.com"
	scope := "fly-print-admin fly-print-operator edge:register edge:printer edge:heartbeat file:read"

	signed, err := generateToken(sub, username, email, scope)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to sign token: %v", err)
		return
	}

	resp := map[string]interface{}{
		"access_token":  signed,
		"token_type":    "bearer",
		"expires_in":    3600,
		"refresh_token": "dummy-refresh-token",
		"id_token":      "dummy-id-token",
		"scope":         scope,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// /userinfo: 返回当前用户信息，供 OAuth2Handler.Me / Verify 使用
func handleUserInfo(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "missing bearer token")
		return
	}

	resp := map[string]interface{}{
		"sub":                "admin-user",
		"preferred_username": "admin",
		"email":              "admin@example.com",
		"name":               "Admin User",
		"realm_access": map[string]interface{}{
			"roles": []string{"fly-print-admin", "fly-print-operator"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

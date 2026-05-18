package authjson

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
)

type Credentials struct {
	APIKey       string
	AccessToken  string
	RefreshToken string
	IDToken      string
	AccountID    string
}

var knownAPIKeyPaths = [][]string{
	{"api_key"},
	{"apiKey"},
	{"openai_api_key"},
	{"openaiApiKey"},
	{"OPENAI_API_KEY"},
	{"openai", "api_key"},
	{"openai", "apiKey"},
}

var stringKeyAliases = map[string][]string{
	"access_token":  {"access_token", "accessToken"},
	"refresh_token": {"refresh_token", "refreshToken"},
	"id_token":      {"id_token", "idToken"},
	"account_id":    {"account_id", "chatgpt_account_id", "chatgptAccountId"},
	"auth_mode":     {"auth_mode", "authMode"},
}

func ReadObject(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func ExtractKnownAPIKey(obj map[string]any) (value, ref string, ok bool) {
	for _, path := range knownAPIKeyPaths {
		if val, ok := lookupStringPath(obj, path...); ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val), strings.Join(path, "."), true
		}
	}
	return "", "", false
}

func ExtractChatGPTCredentials(obj map[string]any) Credentials {
	var tokens map[string]any
	if raw, ok := obj["tokens"].(map[string]any); ok {
		tokens = raw
	}
	creds := Credentials{
		AccessToken:  firstStringInMap(obj, stringKeyAliases["access_token"]...),
		RefreshToken: firstStringInMap(obj, stringKeyAliases["refresh_token"]...),
		IDToken:      firstStringInMap(obj, stringKeyAliases["id_token"]...),
		AccountID:    firstStringInMap(obj, stringKeyAliases["account_id"]...),
	}
	if tokens != nil {
		if creds.AccessToken == "" {
			creds.AccessToken = firstStringInMap(tokens, stringKeyAliases["access_token"]...)
		}
		if creds.RefreshToken == "" {
			creds.RefreshToken = firstStringInMap(tokens, stringKeyAliases["refresh_token"]...)
		}
		if creds.IDToken == "" {
			creds.IDToken = firstStringInMap(tokens, stringKeyAliases["id_token"]...)
		}
		if creds.AccountID == "" {
			creds.AccountID = firstStringInMap(tokens, stringKeyAliases["account_id"]...)
		}
	}
	if creds.AccountID == "" && creds.IDToken != "" {
		creds.AccountID = ChatGPTAccountIDFromJWT(creds.IDToken)
	}
	return creds
}

func ChatGPTAccountIDFromJWT(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		if decoded, err = base64.URLEncoding.DecodeString(parts[1]); err != nil {
			return ""
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if accountID, ok := auth["chatgpt_account_id"].(string); ok {
			return strings.TrimSpace(accountID)
		}
	}
	return ""
}

func firstStringInMap(root map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := root[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func lookupStringPath(root map[string]any, path ...string) (string, bool) {
	var current any = root
	for _, key := range path {
		nextMap, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = nextMap[key]
		if !ok {
			return "", false
		}
	}
	s, ok := current.(string)
	return strings.TrimSpace(s), ok
}

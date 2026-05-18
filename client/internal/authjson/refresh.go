package authjson

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidRefreshPayload = errors.New("authjson: invalid refresh payload")

type RefreshedTokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	AccountID    string
}

func ParseRefreshedTokens(raw []byte) (RefreshedTokens, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return RefreshedTokens{}, err
	}
	out := RefreshedTokens{
		AccessToken:  firstStringInMap(payload, stringKeyAliases["access_token"]...),
		RefreshToken: firstStringInMap(payload, stringKeyAliases["refresh_token"]...),
		IDToken:      firstStringInMap(payload, stringKeyAliases["id_token"]...),
		AccountID:    firstStringInMap(payload, stringKeyAliases["account_id"]...),
	}
	if out.AccountID == "" && out.IDToken != "" {
		out.AccountID = ChatGPTAccountIDFromJWT(out.IDToken)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		return RefreshedTokens{}, ErrInvalidRefreshPayload
	}
	return out, nil
}

func RewriteRefreshedAuthFile(raw []byte, refreshed RefreshedTokens, refreshedAt time.Time) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}

	if mode, ok := root["auth_mode"].(string); !ok || shouldPromoteAuthMode(mode) {
		root["auth_mode"] = "chatgpt"
	}

	if tokens, ok := root["tokens"].(map[string]any); ok {
		tokens["access_token"] = refreshed.AccessToken
		tokens["refresh_token"] = refreshed.RefreshToken
		if refreshed.IDToken != "" {
			tokens["id_token"] = refreshed.IDToken
		}
		if refreshed.AccountID != "" {
			tokens["account_id"] = refreshed.AccountID
		}
		root["tokens"] = tokens
	} else {
		root["access_token"] = refreshed.AccessToken
		root["refresh_token"] = refreshed.RefreshToken
		if refreshed.IDToken != "" {
			root["id_token"] = refreshed.IDToken
		}
		if refreshed.AccountID != "" {
			root["account_id"] = refreshed.AccountID
		}
	}
	root["last_refresh"] = refreshedAt.UTC().Format(time.RFC3339)

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func shouldPromoteAuthMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "apikey", "api_key":
		return true
	default:
		return false
	}
}

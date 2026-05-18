package authjson

import (
	"encoding/json"
	"time"
)

// ReloginData is the auth payload produced by browser relogin.
type ReloginData struct {
	OpenAIAPIKey string
	IDToken      string
	AccessToken  string
	RefreshToken string
	AccountID    string
	RefreshedAt  time.Time
}

// BuildReloginAuthFile creates a supported managed auth.json document.
func BuildReloginAuthFile(data ReloginData) ([]byte, error) {
	root := map[string]any{}
	applyReloginData(root, data)
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// RewriteReloginAuthFile rewrites a supported auth.json document with relogin data.
func RewriteReloginAuthFile(raw []byte, data ReloginData) ([]byte, error) {
	var root map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, err
		}
	}
	if root == nil {
		root = map[string]any{}
	}
	applyReloginData(root, data)
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func applyReloginData(root map[string]any, data ReloginData) {
	root["auth_mode"] = "chatgpt"
	root["OPENAI_API_KEY"] = data.OpenAIAPIKey
	tokens := map[string]any{
		"id_token":      data.IDToken,
		"access_token":  data.AccessToken,
		"refresh_token": data.RefreshToken,
	}
	if data.AccountID != "" {
		tokens["account_id"] = data.AccountID
	}
	root["tokens"] = tokens
	root["last_refresh"] = data.RefreshedAt.UTC().Format(time.RFC3339)
}

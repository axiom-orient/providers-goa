package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/axiom-orient/providers-goa/client/internal/authjson"
)

type authTransport string

const (
	authTransportOpenAI  authTransport = "openai"
	authTransportChatGPT authTransport = "chatgpt"
)

type resolvedAuth struct {
	transport    authTransport
	apiKey       string
	accessToken  string
	refreshToken string
	idToken      string
	accountID    string
	authPath     string
}

// APIKeySource describes where the runtime API key was resolved from.
type APIKeySource string

const (
	APIKeySourceNone     APIKeySource = "none"
	APIKeySourceConfig   APIKeySource = "config"
	APIKeySourceEnv      APIKeySource = "env"
	APIKeySourceAuthFile APIKeySource = "auth_file"
)

// ResolveAuthOptions configures auth path resolution.
type ResolveAuthOptions struct {
	AuthPath string
	AuthHome string
}

// AuthState summarizes local auth discovery without exposing secrets.
type AuthState struct {
	AuthPath               string       `json:"auth_path,omitempty"`
	AuthFileFound          bool         `json:"auth_file_found"`
	AuthFileReadError      string       `json:"auth_file_read_error,omitempty"`
	AuthFileParseError     string       `json:"auth_file_parse_error,omitempty"`
	AuthFileKeys           []string     `json:"auth_file_keys,omitempty"`
	AuthFileKnownAPIKeyRef string       `json:"auth_file_known_api_key_ref,omitempty"`
	AuthFileHasKnownAPIKey bool         `json:"auth_file_has_known_api_key"`
	AuthMode               string       `json:"auth_mode,omitempty"`
	HasAccessToken         bool         `json:"has_access_token"`
	HasRefreshToken        bool         `json:"has_refresh_token"`
	HasIDToken             bool         `json:"has_id_token"`
	HasAccountID           bool         `json:"has_account_id"`
	APIKeySource           APIKeySource `json:"api_key_source"`
	HasAPIKey              bool         `json:"has_api_key"`
	Transport              string       `json:"transport,omitempty"`
}

func (s AuthState) clone() AuthState {
	s.AuthFileKeys = append([]string(nil), s.AuthFileKeys...)
	return s
}

// AuthFileSummary is a parse summary for auth.json.
type AuthFileSummary struct {
	Path            string   `json:"path,omitempty"`
	Exists          bool     `json:"exists"`
	ReadError       string   `json:"read_error,omitempty"`
	ParseError      string   `json:"parse_error,omitempty"`
	TopLevelKeys    []string `json:"top_level_keys,omitempty"`
	KnownAPIKeyRef  string   `json:"known_api_key_ref,omitempty"`
	HasKnownAPIKey  bool     `json:"has_known_api_key"`
	AuthMode        string   `json:"auth_mode,omitempty"`
	HasAccessToken  bool     `json:"has_access_token"`
	HasRefreshToken bool     `json:"has_refresh_token"`
	HasIDToken      bool     `json:"has_id_token"`
	HasAccountID    bool     `json:"has_account_id"`
}

// ResolveAuthPath resolves auth.json path precedence.
func ResolveAuthPath(opts ResolveAuthOptions) (string, error) {
	if opts.AuthPath != "" {
		return filepath.Clean(opts.AuthPath), nil
	}
	if opts.AuthHome != "" {
		return filepath.Join(opts.AuthHome, "auth.json"), nil
	}
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		return filepath.Join(codexHome, "auth.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "auth.json"), nil
}

// InspectAuthFile inspects auth.json and extracts only known, stable fields.
func InspectAuthFile(path string) AuthFileSummary {
	summary := AuthFileSummary{Path: path}
	if path == "" {
		return summary
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return summary
		}
		summary.ReadError = err.Error()
		return summary
	}
	summary.Exists = true

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		summary.ParseError = err.Error()
		return summary
	}

	for k := range obj {
		summary.TopLevelKeys = append(summary.TopLevelKeys, k)
	}
	sort.Strings(summary.TopLevelKeys)

	if _, ref, ok := authjson.ExtractKnownAPIKey(obj); ok {
		summary.HasKnownAPIKey = true
		summary.KnownAPIKeyRef = ref
	}
	authMode, _ := obj["auth_mode"].(string)
	if authMode == "" {
		authMode, _ = obj["authMode"].(string)
	}
	summary.AuthMode = strings.TrimSpace(authMode)
	creds := authjson.ExtractChatGPTCredentials(obj)
	summary.HasAccessToken = strings.TrimSpace(creds.AccessToken) != ""
	summary.HasRefreshToken = strings.TrimSpace(creds.RefreshToken) != ""
	summary.HasIDToken = strings.TrimSpace(creds.IDToken) != ""
	summary.HasAccountID = strings.TrimSpace(creds.AccountID) != ""
	return summary
}

func resolveAuthState(cfg Config) (AuthState, resolvedAuth, error) {
	cfg = cfg.normalized()
	state := AuthState{APIKeySource: APIKeySourceNone}
	resolved := resolvedAuth{transport: authTransportOpenAI}

	authPath, err := ResolveAuthPath(ResolveAuthOptions{AuthPath: cfg.AuthPath, AuthHome: cfg.AuthHome})
	if err != nil {
		state.AuthFileReadError = err.Error()
	} else {
		state.AuthPath = authPath
		resolved.authPath = authPath
		summary := InspectAuthFile(authPath)
		state.AuthFileFound = summary.Exists
		state.AuthFileReadError = summary.ReadError
		state.AuthFileParseError = summary.ParseError
		state.AuthFileKeys = append([]string(nil), summary.TopLevelKeys...)
		state.AuthFileKnownAPIKeyRef = summary.KnownAPIKeyRef
		state.AuthFileHasKnownAPIKey = summary.HasKnownAPIKey
		state.AuthMode = summary.AuthMode
		state.HasAccessToken = summary.HasAccessToken
		state.HasRefreshToken = summary.HasRefreshToken
		state.HasIDToken = summary.HasIDToken
		state.HasAccountID = summary.HasAccountID
	}

	if cfg.PreferChatGPT {
		resolved.transport = authTransportChatGPT
		state.Transport = string(authTransportChatGPT)
		if state.AuthFileFound && state.AuthFileReadError == "" && state.AuthFileParseError == "" {
			obj, readErr := authjson.ReadObject(state.AuthPath)
			if readErr == nil {
				creds := authjson.ExtractChatGPTCredentials(obj)
				if strings.TrimSpace(creds.AccessToken) != "" || strings.TrimSpace(creds.RefreshToken) != "" {
					resolved.accessToken = strings.TrimSpace(creds.AccessToken)
					resolved.refreshToken = strings.TrimSpace(creds.RefreshToken)
					resolved.idToken = strings.TrimSpace(creds.IDToken)
					resolved.accountID = strings.TrimSpace(creds.AccountID)
				}
			}
		}
		return state, resolved, nil
	}

	if cfg.APIKey != "" {
		state.APIKeySource = APIKeySourceConfig
		state.HasAPIKey = true
		state.Transport = string(authTransportOpenAI)
		resolved.apiKey = cfg.APIKey
		return state, resolved, nil
	}

	if cfg.APIKeyEnv != "" {
		envKey := os.Getenv(cfg.APIKeyEnv)
		if envKey != "" {
			state.APIKeySource = APIKeySourceEnv
			state.HasAPIKey = true
			state.Transport = string(authTransportOpenAI)
			resolved.apiKey = envKey
			return state, resolved, nil
		}
	}

	if state.AuthFileFound && state.AuthFileReadError == "" && state.AuthFileParseError == "" {
		obj, readErr := authjson.ReadObject(state.AuthPath)
		if readErr == nil {
			if apiKey, _, ok := authjson.ExtractKnownAPIKey(obj); ok && strings.TrimSpace(apiKey) != "" {
				state.APIKeySource = APIKeySourceAuthFile
				state.HasAPIKey = true
				state.Transport = string(authTransportOpenAI)
				resolved.apiKey = apiKey
				return state, resolved, nil
			}
			creds := authjson.ExtractChatGPTCredentials(obj)
			if strings.TrimSpace(creds.AccessToken) != "" || strings.TrimSpace(creds.RefreshToken) != "" {
				state.Transport = string(authTransportChatGPT)
				resolved.transport = authTransportChatGPT
				resolved.accessToken = strings.TrimSpace(creds.AccessToken)
				resolved.refreshToken = strings.TrimSpace(creds.RefreshToken)
				resolved.idToken = strings.TrimSpace(creds.IDToken)
				resolved.accountID = strings.TrimSpace(creds.AccountID)
				return state, resolved, nil
			}
		}
	}

	state.Transport = string(resolved.transport)
	return state, resolved, nil
}

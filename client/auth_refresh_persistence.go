package client

import (
	"time"

	"github.com/axiom-orient/providers-goa/client/internal/authjson"
)

func persistRefreshedChatGPTTokens(path, accessToken, refreshToken, idToken, accountID string) error {
	if path == "" {
		return &ValidationError{Field: "path", Message: "must not be empty"}
	}
	refreshed := authjson.RefreshedTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IDToken:      idToken,
		AccountID:    accountID,
	}
	return withAuthRefreshFileLock(path, func() error {
		raw, err := ReadAuthFileBytes(path)
		if err != nil {
			return err
		}
		encoded, err := rewriteRefreshedChatGPTAuthFile(raw, refreshed, time.Now().UTC())
		if err != nil {
			return err
		}
		return authjson.WritePrivateFile(path, encoded)
	})
}

func rewriteRefreshedChatGPTAuthFile(raw []byte, refreshed authjson.RefreshedTokens, refreshedAt time.Time) ([]byte, error) {
	return authjson.RewriteRefreshedAuthFile(raw, refreshed, refreshedAt)
}

package appserver

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	clientpkg "github.com/axiom-orient/providers-goa/client"
)

const refreshHandlerTimeout = 10 * time.Second

type initializeParams struct {
	ClientInfo   ClientInfo           `json:"clientInfo"`
	Capabilities *initializeCapsUnion `json:"capabilities,omitempty"`
}

type initializeCapsUnion struct {
	ExperimentalAPI           bool     `json:"experimentalApi,omitempty"`
	OptOutNotificationMethods []string `json:"optOutNotificationMethods,omitempty"`
}

func (c *Client) handleServerRequest(env rpcEnvelope) {
	switch env.Method {
	case "account/chatgptAuthTokens/refresh":
		var req ChatGPTAuthTokensRefreshRequest
		if err := json.Unmarshal(env.Params, &req); err != nil {
			_ = c.send(rpcEnvelope{ID: cloneBytes(env.ID), Error: &RPCError{Code: -32602, Message: err.Error()}})
			return
		}
		if c.refreshHandler == nil {
			_ = c.send(rpcEnvelope{ID: cloneBytes(env.ID), Error: &RPCError{Code: -32601, Message: "no refresh handler configured"}})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), refreshHandlerTimeout)
		defer cancel()
		result, err := c.refreshHandler(ctx, req)
		if err != nil {
			_ = c.send(rpcEnvelope{ID: cloneBytes(env.ID), Error: &RPCError{Code: -32000, Message: err.Error()}})
			return
		}
		_ = c.send(map[string]any{"id": decodeRawID(env.ID), "result": result})
	default:
		_ = c.send(rpcEnvelope{ID: cloneBytes(env.ID), Error: &RPCError{Code: -32601, Message: "method not supported by goa appserver client"}})
	}
}

func normalizeClientInfo(info ClientInfo) ClientInfo {
	if strings.TrimSpace(info.Name) == "" {
		info.Name = "goa"
	}
	if strings.TrimSpace(info.Title) == "" {
		info.Title = "goa"
	}
	if strings.TrimSpace(info.Version) == "" {
		info.Version = clientpkg.BuildInfoSnapshot().Version
	}
	return info
}

func normalizeCapabilities(caps Capabilities) *initializeCapsUnion {
	if !caps.ExperimentalAPI && len(caps.OptOutNotificationMethods) == 0 {
		return nil
	}
	return &initializeCapsUnion{ExperimentalAPI: caps.ExperimentalAPI, OptOutNotificationMethods: append([]string(nil), caps.OptOutNotificationMethods...)}
}

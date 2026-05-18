package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ModelsService wraps /v1/models.
type ModelsService struct {
	client *Client
}

// List calls GET /v1/models.
func (s *ModelsService) List(ctx context.Context) (ListModelsResponse, error) {
	var out ListModelsResponse
	if err := s.client.ensureNoActiveSend(); err != nil {
		return out, err
	}
	if err := s.client.requireCredential(); err != nil {
		return out, err
	}
	var req *http.Request
	var err error
	if s.client.isChatGPTTransport() {
		if err := s.client.ensureChatGPTAccessToken(ctx); err != nil {
			return out, err
		}
		req, err = s.client.buildChatGPTModelsRequest(ctx)
	} else {
		req, err = s.client.newRequest(ctx, http.MethodGet, "/v1/models", nil)
	}
	if err != nil {
		return out, err
	}
	raw, meta, err := s.client.doRawJSON(req)
	if err != nil && s.client.isChatGPTTransport() && isAPIStatus(err, http.StatusUnauthorized) && s.client.hasChatGPTRefreshToken() {
		if refreshErr := s.client.refreshChatGPTAuth(ctx); refreshErr != nil {
			return out, refreshErr
		}
		req, err = s.client.buildChatGPTModelsRequest(ctx)
		if err != nil {
			return out, err
		}
		raw, meta, err = s.client.doRawJSON(req)
	}
	if err != nil {
		return out, err
	}
	if err := parseModelsPayload(raw, &out); err != nil {
		return out, err
	}
	out.Meta = meta
	return out, nil
}

func parseModelsPayload(raw []byte, out *ListModelsResponse) error {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("goa: decode response: %w", err)
	}
	out.Object = strings.TrimSpace(fmt.Sprint(payload["object"]))
	if data, ok := payload["data"].([]any); ok {
		out.Data = decodeOpenAIModels(data)
		return nil
	}
	if models, ok := payload["models"].([]any); ok {
		out.Object = "list"
		out.Data = decodeChatGPTModels(models)
		return nil
	}
	return fmt.Errorf("goa: decode response: invalid models response")
}

func decodeOpenAIModels(items []any) []Model {
	out := make([]Model, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := obj["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		object, _ := obj["object"].(string)
		ownedBy, _ := obj["owned_by"].(string)
		out = append(out, Model{
			ID:      id,
			Object:  strings.TrimSpace(object),
			OwnedBy: strings.TrimSpace(ownedBy),
		})
	}
	return out
}

func decodeChatGPTModels(items []any) []Model {
	out := make([]Model, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := obj["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			slug, _ := obj["slug"].(string)
			id = strings.TrimSpace(slug)
		}
		if id == "" {
			continue
		}
		ownedBy, _ := obj["owned_by"].(string)
		out = append(out, Model{ID: id, Object: "model", OwnedBy: strings.TrimSpace(ownedBy)})
	}
	return out
}

package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	DefaultGeminiNodePath    = "node"
	DefaultGeminiAdapterPath = "gemini-core-adapter/dist/main.js"
)

var ErrGeminiMissingPrompt = errors.New("goa: gemini prompt is required")

type GeminiGenerateRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
}

type GeminiGenerateResponse struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type GeminiModelQuota struct {
	RemainingAmount   *uint64  `json:"remainingAmount,omitempty"`
	RemainingFraction *float64 `json:"remainingFraction,omitempty"`
	ResetTime         string   `json:"resetTime,omitempty"`
}

type GeminiModelInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tier        string            `json:"tier"`
	Source      string            `json:"source"`
	Quota       *GeminiModelQuota `json:"quota,omitempty"`
}

type GeminiModelsResponse struct {
	Provider       string            `json:"provider"`
	ReleaseChannel string            `json:"releaseChannel"`
	Models         []GeminiModelInfo `json:"models"`
}

type GeminiClient struct {
	NodePath    string
	AdapterPath string
}

func NewGeminiClient(nodePath, adapterPath string) GeminiClient {
	if strings.TrimSpace(nodePath) == "" {
		nodePath = DefaultGeminiNodePath
	}
	if strings.TrimSpace(adapterPath) == "" {
		adapterPath = DefaultGeminiAdapterPath
	}
	return GeminiClient{NodePath: nodePath, AdapterPath: adapterPath}
}

func (c GeminiClient) Generate(request GeminiGenerateRequest) (GeminiGenerateResponse, error) {
	var out GeminiGenerateResponse
	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" {
		return out, ErrGeminiMissingPrompt
	}
	params := map[string]any{"prompt": prompt}
	if model := strings.TrimSpace(request.Model); model != "" {
		params["model"] = model
	}
	err := c.callAdapter(map[string]any{
		"id":     "1",
		"method": "generate",
		"params": params,
	}, &out)
	return out, err
}

func (c GeminiClient) Models() (GeminiModelsResponse, error) {
	var out GeminiModelsResponse
	err := c.callAdapter(map[string]any{
		"id":     "1",
		"method": "models",
		"params": map[string]any{},
	}, &out)
	return out, err
}

func (c GeminiClient) callAdapter(payload map[string]any, out any) error {
	if _, err := os.Stat(c.AdapterPath); err != nil {
		return fmt.Errorf("goa: Gemini Core Adapter not found at %s; run `cd gemini-core-adapter && npm install && npm run build`: %w", c.AdapterPath, err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cmd := exec.Command(c.NodePath, c.AdapterPath)
	cmd.Stdin = bytes.NewReader(append(body, '\n'))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("goa: Gemini Core Adapter failed: %s", sanitizePublicMessage(msg))
	}
	return DecodeGeminiAdapterResponse(stdout.Bytes(), out)
}

func DecodeGeminiAdapterResponse(data []byte, out any) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var lastErr error
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var envelope struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			lastErr = err
			continue
		}
		if envelope.Error != nil {
			return fmt.Errorf("goa: Gemini Core Adapter RPC error code=%d message=%s", envelope.Error.Code, sanitizePublicMessage(envelope.Error.Message))
		}
		if len(envelope.Result) == 0 {
			return errors.New("goa: Gemini Core Adapter returned an empty response")
		}
		return json.Unmarshal(envelope.Result, out)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if lastErr != nil {
		return fmt.Errorf("goa: decode Gemini Core Adapter response: %w", lastErr)
	}
	return errors.New("goa: Gemini Core Adapter returned an empty response")
}

func sanitizePublicMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 400 {
		return value[:400] + "..."
	}
	return value
}

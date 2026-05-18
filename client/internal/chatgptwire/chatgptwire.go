package chatgptwire

import (
	"encoding/json"
	"fmt"
	"strings"
)

type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	if e == nil {
		return ""
	}
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

type Request struct {
	Model             string
	Input             any
	Instructions      string
	ParallelToolCalls bool
	ToolChoice        any
	Tools             []map[string]any
	Reasoning         any
	Text              any
	Extra             map[string]any
}

func MarshalResponseRequest(in Request) ([]byte, error) {
	input, hoistedInstructions, err := NormalizeInput(in.Input)
	if err != nil {
		return nil, err
	}
	instructions := strings.Join(nonEmptyStrings(in.Instructions, hoistedInstructions), "\n\n")

	toolChoice := any("auto")
	if in.ToolChoice != nil {
		toolChoice = in.ToolChoice
	}

	body := map[string]any{
		"stream":              true,
		"instructions":        instructions,
		"parallel_tool_calls": in.ParallelToolCalls,
		"tool_choice":         toolChoice,
		"tools":               encodeTools(in.Tools),
		"include":             []any{},
		"store":               false,
	}
	if in.Model != "" {
		body["model"] = in.Model
	}
	body["input"] = input
	if reasoning := normalizeReasoning(in.Reasoning); reasoning != nil {
		body["reasoning"] = reasoning
	}
	if in.Text != nil {
		body["text"] = in.Text
	}
	for key, value := range in.Extra {
		if _, exists := body[key]; !exists {
			body[key] = value
		}
	}
	return json.Marshal(body)
}

func NormalizeInput(value any) ([]any, string, error) {
	if value == nil {
		return []any{}, "", nil
	}
	switch raw := value.(type) {
	case string:
		return []any{message("user", []any{map[string]any{"type": "input_text", "text": raw}})}, "", nil
	case []any:
		return NormalizeInputItems(raw)
	default:
		blob, err := json.Marshal(value)
		if err != nil {
			return nil, "", err
		}
		var generic any
		if err := json.Unmarshal(blob, &generic); err != nil {
			return nil, "", err
		}
		switch v := generic.(type) {
		case string:
			return []any{message("user", []any{map[string]any{"type": "input_text", "text": v}})}, "", nil
		case []any:
			return NormalizeInputItems(v)
		default:
			return nil, "", &FieldError{Field: "input", Message: "must be a string or array"}
		}
	}
}

func NormalizeInputItems(items []any) ([]any, string, error) {
	out := make([]any, 0, len(items))
	var instructions []string
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, "", &FieldError{Field: "input", Message: "array items must be objects"}
		}
		role := strings.TrimSpace(fmt.Sprint(obj["role"]))
		if role == "" {
			role = "user"
		}
		content, err := NormalizeContent(obj["content"], role)
		if err != nil {
			return nil, "", err
		}
		if role == "system" {
			text, err := FlattenText(content)
			if err != nil {
				return nil, "", err
			}
			instructions = append(instructions, text)
			continue
		}
		out = append(out, message(role, content))
	}
	return out, strings.Join(instructions, "\n\n"), nil
}

func NormalizeContent(value any, role string) ([]any, error) {
	textType := "input_text"
	if role == "assistant" {
		textType = "output_text"
	}
	switch raw := value.(type) {
	case nil:
		return []any{}, nil
	case string:
		return []any{map[string]any{"type": textType, "text": raw}}, nil
	case []any:
		out := make([]any, 0, len(raw))
		for index, item := range raw {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, &FieldError{Field: "input.content", Message: fmt.Sprintf("item %d must be an object", index)}
			}
			typeName, err := contentTypeName(obj)
			if err != nil {
				return nil, err
			}
			switch typeName {
			case "", "text", "input_text", "output_text":
				text, err := normalizeTextPart(obj, textType)
				if err != nil {
					return nil, err
				}
				out = append(out, text)
			case "image_url", "input_image":
				image, err := normalizeImagePart(obj)
				if err != nil {
					return nil, err
				}
				out = append(out, image)
			default:
				return nil, &FieldError{Field: "input.content", Message: fmt.Sprintf("unsupported content type %q", typeName)}
			}
		}
		return out, nil
	default:
		return nil, &FieldError{Field: "input.content", Message: "must be a string or array"}
	}
}

func FlattenText(parts []any) (string, error) {
	texts := make([]string, 0, len(parts))
	for index, part := range parts {
		obj, ok := part.(map[string]any)
		if !ok {
			return "", &FieldError{Field: "input.content", Message: fmt.Sprintf("item %d must be an object", index)}
		}
		typeName, err := contentTypeName(obj)
		if err != nil {
			return "", err
		}
		switch typeName {
		case "", "text", "input_text", "output_text":
		default:
			return "", &FieldError{Field: "input.content", Message: fmt.Sprintf("system role does not support content type %q", typeName)}
		}
		text, found, err := firstExactString(obj, "text", "value")
		if err != nil {
			return "", err
		}
		if !found {
			return "", &FieldError{Field: "input.content", Message: "text content requires text or value string"}
		}
		texts = append(texts, text)
	}
	return strings.Join(texts, "\n"), nil
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func encodeTools(tools []map[string]any) []any {
	out := make([]any, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(fmt.Sprint(tool["name"]))
		if name == "" {
			continue
		}
		typeName := strings.TrimSpace(fmt.Sprint(tool["type"]))
		if typeName == "" {
			typeName = "function"
		}
		encoded := map[string]any{
			"type": typeName,
			"name": name,
		}
		if description := strings.TrimSpace(fmt.Sprint(tool["description"])); description != "" {
			encoded["description"] = description
		}
		if parameters, ok := tool["parameters"].(map[string]any); ok {
			encoded["parameters"] = parameters
		}
		if strict, _ := tool["strict"].(bool); strict {
			encoded["strict"] = true
		}
		out = append(out, encoded)
	}
	return out
}

func normalizeReasoning(value any) map[string]any {
	if value == nil {
		return nil
	}
	effort := strings.TrimSpace(fmt.Sprint(value))
	if effort == "" {
		return nil
	}
	if raw, ok := value.(map[string]any); ok {
		effort = strings.TrimSpace(fmt.Sprint(raw["effort"]))
	}
	if effort == "" {
		return nil
	}
	return map[string]any{
		"effort":  normalizeEffort(effort),
		"summary": "auto",
	}
}

func normalizeEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none", "minimal":
		return "low"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func message(role string, content []any) map[string]any {
	return map[string]any{
		"type":    "message",
		"role":    role,
		"content": content,
	}
}

func contentTypeName(obj map[string]any) (string, error) {
	raw, ok := obj["type"]
	if !ok || raw == nil {
		return "", nil
	}
	typeName, ok := raw.(string)
	if !ok {
		return "", &FieldError{Field: "input.content", Message: "type must be a string"}
	}
	return strings.TrimSpace(typeName), nil
}

func normalizeTextPart(obj map[string]any, textType string) (map[string]any, error) {
	text, found, err := firstExactString(obj, "text", "value")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, &FieldError{Field: "input.content", Message: "text content requires text or value string"}
	}
	return map[string]any{"type": textType, "text": text}, nil
}

func normalizeImagePart(obj map[string]any) (map[string]any, error) {
	urlValue, found, err := firstExactString(obj, "url")
	if err != nil {
		return nil, err
	}
	if !found {
		nestedRaw, ok := obj["image_url"]
		if !ok || nestedRaw == nil {
			return nil, &FieldError{Field: "input.content", Message: "image content requires url or image_url.url string"}
		}
		nested, ok := nestedRaw.(map[string]any)
		if !ok {
			return nil, &FieldError{Field: "input.content", Message: "image_url must be an object"}
		}
		urlValue, found, err = firstExactString(nested, "url")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, &FieldError{Field: "input.content", Message: "image content requires url or image_url.url string"}
		}
	}
	return map[string]any{"type": "input_image", "image_url": urlValue}, nil
}

func firstExactString(obj map[string]any, keys ...string) (string, bool, error) {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return "", false, &FieldError{Field: "input.content", Message: fmt.Sprintf("%s must be a string", key)}
		}
		return text, true, nil
	}
	return "", false, nil
}

package client

import "regexp"

const (
	maxMetadataPairs       = 16
	maxMetadataKeyLength   = 64
	maxMetadataValueLength = 512
)

var (
	reservedCreateResponseExtraKeys = map[string]struct{}{
		"model":        {},
		"input":        {},
		"instructions": {},
		"stream":       {},
		"prompt":       {},
		"reasoning":    {},
		"text":         {},
		"metadata":     {},
	}
	responseFormatNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

func (r CreateResponseRequest) validateForCreate() error {
	return r.validateWithStream(false)
}

func (r CreateResponseRequest) validateForStream() error {
	return r.validateWithStream(true)
}

func (r CreateResponseRequest) validateWithStream(stream bool) error {
	for key := range r.Extra {
		if _, reserved := reservedCreateResponseExtraKeys[key]; reserved {
			return &ValidationError{
				Field:   "extra." + key,
				Message: "reserved key is controlled by the typed request surface",
			}
		}
	}
	if err := validateResponseTextConfig(r.Text); err != nil {
		return err
	}
	if len(r.Metadata) > maxMetadataPairs {
		return &ValidationError{
			Field:   "metadata",
			Message: "must contain at most 16 key-value pairs",
		}
	}
	for key, value := range r.Metadata {
		if len(key) > maxMetadataKeyLength {
			return &ValidationError{
				Field:   "metadata." + key,
				Message: "key length must be at most 64 characters",
			}
		}
		if len(value) > maxMetadataValueLength {
			return &ValidationError{
				Field:   "metadata." + key,
				Message: "value length must be at most 512 characters",
			}
		}
	}
	if _, hasPreviousResponseID := r.Extra["previous_response_id"]; hasPreviousResponseID {
		if _, hasConversation := r.Extra["conversation"]; hasConversation {
			return &ValidationError{
				Field:   "extra.previous_response_id",
				Message: "cannot be used with conversation",
			}
		}
	}
	if _, hasStreamOptions := r.Extra["stream_options"]; hasStreamOptions && !stream {
		return &ValidationError{
			Field:   "extra.stream_options",
			Message: "requires stream mode",
		}
	}
	return nil
}

func validateResponseTextConfig(cfg *ResponseTextConfig) error {
	if cfg == nil {
		return nil
	}
	if cfg.Verbosity != "" {
		switch cfg.Verbosity {
		case "low", "medium", "high":
		default:
			return &ValidationError{
				Field:   "text.verbosity",
				Message: "must be one of low, medium, or high",
			}
		}
	}
	if cfg.Format == nil {
		return nil
	}
	format := cfg.Format
	switch format.Type {
	case "", ResponseTextFormatTypeText:
		if format.Name != "" || format.Description != "" || format.Schema != nil || format.Strict {
			return &ValidationError{
				Field:   "text.format",
				Message: "text format cannot include json_schema-specific fields",
			}
		}
		return nil
	case ResponseTextFormatTypeJSONObject:
		if format.Name != "" || format.Description != "" || format.Schema != nil || format.Strict {
			return &ValidationError{
				Field:   "text.format",
				Message: "json_object format cannot include json_schema-specific fields",
			}
		}
		return nil
	case ResponseTextFormatTypeJSONSchema:
		if !responseFormatNamePattern.MatchString(format.Name) {
			return &ValidationError{
				Field:   "text.format.name",
				Message: "must contain only letters, numbers, underscores, or dashes and be at most 64 characters",
			}
		}
		if format.Schema == nil {
			return &ValidationError{
				Field:   "text.format.schema",
				Message: "is required for json_schema format",
			}
		}
		return nil
	default:
		return &ValidationError{
			Field:   "text.format.type",
			Message: "must be one of text, json_schema, or json_object",
		}
	}
}

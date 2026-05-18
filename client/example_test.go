package client_test

import (
	"fmt"
	"log"

	goa "github.com/axiom-orient/providers-goa/client"
)

type calendarEvent struct {
	Name string `json:"name"`
	Day  string `json:"day"`
}

func ExampleJSONSchemaTextFormat() {
	cfg := goa.JSONSchemaTextFormat("calendar_event", map[string]any{
		"type": "object",
	}, goa.JSONSchemaFormatOptions{Strict: true})
	fmt.Println(cfg.Format.Type, cfg.Format.Name, cfg.Format.Strict)
	// Output: json_schema calendar_event true
}

func ExampleDecodeStructuredOutput() {
	resp := goa.Response{
		Output: []goa.ResponseOutputItem{{
			Type: "message",
			Role: "assistant",
			Content: []goa.ResponseContentPart{{
				Type: "output_text",
				Text: `{"name":"standup","day":"monday"}`,
			}},
		}},
	}

	event, err := goa.DecodeStructuredOutput[calendarEvent](resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(event.Name, event.Day)
	// Output: standup monday
}

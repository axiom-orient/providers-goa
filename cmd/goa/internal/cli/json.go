package cli

import (
	"encoding/json"
	"fmt"
)

func (a *App) writeJSON(v any) int {
	enc := json.NewEncoder(a.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	return 0
}

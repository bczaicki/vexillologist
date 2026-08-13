// Package output renders CLI results as JSON (for agents/scripts) or tables
// (for humans).
package output

import (
	"encoding/json"
	"io"
)

// JSON writes v as indented JSON followed by a newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// MutationResult is the stable envelope for successful write operations.
type MutationResult struct {
	OK          bool   `json:"ok"`
	Action      string `json:"action"`
	Environment string `json:"environment,omitempty"`
	Critical    bool   `json:"critical,omitempty"`
	DryRun      bool   `json:"dryRun,omitempty"`
	Flag        any    `json:"flag,omitempty"`
	Patch       any    `json:"patch,omitempty"`
}

// ErrorResult is the stable envelope for failures when --json is set.
type ErrorResult struct {
	OK    bool        `json:"ok"`
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries the machine-readable error code and human message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

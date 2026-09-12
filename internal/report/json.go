package report

import (
	"encoding/json"
	"io"

	"ci-artifact-identity/internal/analysis"
)

// RenderJSON writes deterministic machine-readable JSON to the given writer.
func RenderJSON(w io.Writer, res *analysis.AnalysisResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(res)
}

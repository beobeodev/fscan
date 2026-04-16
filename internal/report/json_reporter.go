package report

import (
	"encoding/json"
	"io"
	"time"
)

// JSONReporter writes a JSON report.
type JSONReporter struct{}

type jsonReport struct {
	Project   string   `json:"project"`
	ScannedAt string   `json:"scanned_at"`
	Total     int      `json:"total"`
	Issues    []*Issue `json:"issues"`
}

func (r *JSONReporter) Report(issues []*Issue, projectRoot string, w io.Writer) error {
	rep := jsonReport{
		Project:   projectRoot,
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
		Total:     len(issues),
		Issues:    issues,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

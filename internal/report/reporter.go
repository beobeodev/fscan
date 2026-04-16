package report

import (
	"io"
)

// Severity of a reported issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Issue is a single finding from the scanner.
type Issue struct {
	// Rule is the rule ID that produced this issue.
	Rule string `json:"rule"`

	// File is the path relative to project root.
	File string `json:"file"`

	// Line is the declaration line (0 = file-level).
	Line int `json:"line"`

	// Col is the column (0 = file-level).
	Col int `json:"col"`

	// Message is the human-readable description.
	Message string `json:"message"`

	// Severity is the issue severity.
	Severity Severity `json:"severity"`
}

// Reporter writes a list of issues to an io.Writer.
type Reporter interface {
	Report(issues []*Issue, projectRoot string, w io.Writer) error
}

// New returns a reporter for the given format ("text", "json", "sarif").
func New(format string) Reporter {
	switch format {
	case "json":
		return &JSONReporter{}
	case "sarif":
		return &SARIFReporter{}
	default:
		return &TextReporter{}
	}
}

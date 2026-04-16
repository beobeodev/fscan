package report

import (
	"encoding/json"
	"io"
)

// SARIFReporter writes a SARIF 2.1.0 report for GitHub Code Scanning / VS Code.
type SARIFReporter struct{}

const sarifVersion = "2.1.0"
const sarifSchema = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"

type sarifDoc struct {
	Schema  string      `json:"$schema"`
	Version string      `json:"version"`
	Runs    []sarifRun  `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool    `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name    string       `json:"name"`
	Version string       `json:"version"`
	Rules   []sarifRule  `json:"rules"`
}

type sarifRule struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	ShortDescription sarifMessage           `json:"shortDescription"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

var ruleDescriptions = map[string]string{
	"unused-file":              "Dart file not reachable from any entry point",
	"unused-private-class":     "Private class with no references within the package",
	"unused-private-function":  "Private function or method with no references within the package",
	"unused-asset":             "Asset declared in pubspec.yaml but never referenced",
	"maybe-unused-public-api":  "Public symbol with no references within the project",
	"maybe-unused-widget":      "Widget subclass not instantiated or registered in routes",
}

func (r *SARIFReporter) Report(issues []*Issue, projectRoot string, w io.Writer) error {
	// Build rules list from unique rule IDs in issues
	ruleMap := make(map[string]struct{})
	for _, issue := range issues {
		ruleMap[issue.Rule] = struct{}{}
	}

	var sarifRules []sarifRule
	for ruleID := range ruleMap {
		desc, ok := ruleDescriptions[ruleID]
		if !ok {
			desc = ruleID
		}
		sarifRules = append(sarifRules, sarifRule{
			ID:               ruleID,
			Name:             toCamelCase(ruleID),
			ShortDescription: sarifMessage{Text: desc},
		})
	}

	var results []sarifResult
	for _, issue := range issues {
		line := issue.Line
		if line <= 0 {
			line = 1
		}
		level := "error"
		if issue.Severity == SeverityWarning {
			level = "warning"
		} else if issue.Severity == SeverityInfo {
			level = "note"
		}

		results = append(results, sarifResult{
			RuleID:  issue.Rule,
			Level:   level,
			Message: sarifMessage{Text: issue.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: issue.File},
					Region:           sarifRegion{StartLine: line},
				},
			}},
		})
	}

	doc := sarifDoc{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:    "fscan",
					Version: "0.1.0",
					Rules:   sarifRules,
				},
			},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func toCamelCase(s string) string {
	var result []byte
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' {
			upper = true
			continue
		}
		if upper {
			if c >= 'a' && c <= 'z' {
				c -= 32
			}
			upper = false
		}
		result = append(result, c)
	}
	return string(result)
}

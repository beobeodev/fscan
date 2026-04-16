package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// TextReporter writes a human-readable plain-text report.
type TextReporter struct{}

func (r *TextReporter) Report(issues []*Issue, projectRoot string, w io.Writer) error {
	if len(issues) == 0 {
		fmt.Fprintln(w, "fscan: no issues found.")
		return nil
	}

	fmt.Fprintf(w, "fscan results for: %s\n", projectRoot)
	fmt.Fprintln(w, strings.Repeat("─", 60))

	// Group by rule
	grouped := make(map[string][]*Issue)
	for _, issue := range issues {
		grouped[issue.Rule] = append(grouped[issue.Rule], issue)
	}

	// Sort rule groups for deterministic output
	rules := make([]string, 0, len(grouped))
	for rule := range grouped {
		rules = append(rules, rule)
	}
	sort.Strings(rules)

	for _, rule := range rules {
		ruleIssues := grouped[rule]
		fmt.Fprintf(w, "\n%s (%d)\n", strings.ToUpper(strings.ReplaceAll(rule, "-", " ")), len(ruleIssues))
		for _, issue := range ruleIssues {
			if issue.Line > 0 {
				fmt.Fprintf(w, "  %s:%d  %s\n", issue.File, issue.Line, issue.Message)
			} else {
				fmt.Fprintf(w, "  %s\n", issue.File)
			}
		}
	}

	errors, warnings := 0, 0
	for _, i := range issues {
		if i.Severity == SeverityError {
			errors++
		} else if i.Severity == SeverityWarning {
			warnings++
		}
	}

	fmt.Fprintln(w, "\n"+strings.Repeat("─", 60))
	fmt.Fprintf(w, "Total: %d issues (%d errors, %d warnings)\n", len(issues), errors, warnings)
	fmt.Fprintln(w, "Run with --format json for machine-readable output.")
	return nil
}

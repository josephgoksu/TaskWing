// Package ui provides rendering for drift detection results.
package ui

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/app"
)

// RenderDriftReport renders a drift analysis report to the terminal.
func RenderDriftReport(report *app.DriftReport, verbose bool) {
	if report == nil {
		fmt.Println("No drift report available.")
		return
	}

	// No rules found
	if report.RulesChecked == 0 {
		fmt.Println("📋 No architectural rules found in knowledge base.")
		fmt.Println("   Run 'taskwing bootstrap' to extract rules from your codebase,")
		fmt.Println("   or refresh rules with 'taskwing bootstrap --force'")
		return
	}

	// Violations
	if len(report.Violations) > 0 {
		fmt.Printf("❌ %s (%d)\n", StyleBold("VIOLATIONS"), len(report.Violations))
		fmt.Println("────────────────────────")
		fmt.Println()

		// Group by rule
		byRule := groupViolationsByRule(report.Violations)
		for ruleName, violations := range byRule {
			fmt.Printf("   %s %s\n", StyleBold("Rule:"), ruleName)
			if len(violations) > 0 && violations[0].Rule != nil {
				desc := truncate(violations[0].Rule.Description, 80)
				if desc != "" {
					fmt.Printf("   %s\n", desc)
				}
			}
			fmt.Println()

			for i, v := range violations {
				if i >= 5 && !verbose {
					fmt.Printf("   ... and %d more (use --verbose to see all)\n", len(violations)-5)
					break
				}
				renderViolation(v, i+1)
			}
			fmt.Println()
		}
	}

	// Warnings
	if len(report.Warnings) > 0 {
		fmt.Printf("⚠️  %s (%d)\n", StyleBold("WARNINGS"), len(report.Warnings))
		fmt.Println("────────────────────────")
		fmt.Println()

		for i, w := range report.Warnings {
			if i >= 3 && !verbose {
				fmt.Printf("   ... and %d more (use --verbose to see all)\n", len(report.Warnings)-3)
				break
			}
			renderViolation(w, i+1)
		}
		fmt.Println()
	}

	// Passed rules
	if len(report.Passed) > 0 {
		fmt.Printf("✅ %s (%d)\n", StyleBold("PASSED"), len(report.Passed))
		fmt.Println("────────────────────────")
		for _, name := range report.Passed {
			fmt.Printf("   ✓ %s\n", name)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("────────────────────────")
	fmt.Printf("📊 %s: ", StyleBold("Summary"))

	parts := []string{}
	if report.Summary.Violations > 0 {
		parts = append(parts, fmt.Sprintf("%d violations", report.Summary.Violations))
	}
	if report.Summary.Warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warnings", report.Summary.Warnings))
	}
	if report.Summary.Passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", report.Summary.Passed))
	}

	if len(parts) == 0 {
		fmt.Println("no rules checked")
	} else {
		fmt.Println(strings.Join(parts, ", "))
	}

	// Hint for fixes
	if report.Summary.Violations > 0 {
		fmt.Println()
		fmt.Println("💡 Review violations and update code to match documented architecture.")
	}
}

// renderViolation renders a single violation.
func renderViolation(v app.Violation, num int) {
	fmt.Printf("   %d. %s\n", num, v.Location)
	if v.Symbol != nil {
		fmt.Printf("      ├─ Function: %s\n", v.Symbol.Name)
	}
	fmt.Printf("      ├─ Issue: %s\n", v.Message)
	if v.Evidence != "" {
		fmt.Printf("      ├─ Evidence: %s\n", v.Evidence)
	}
	if v.Suggestion != "" {
		fmt.Printf("      └─ Suggestion: %s\n", v.Suggestion)
	} else {
		fmt.Printf("      └─ Severity: %s\n", v.Severity)
	}
}

// groupViolationsByRule groups violations by their rule name.
func groupViolationsByRule(violations []app.Violation) map[string][]app.Violation {
	result := make(map[string][]app.Violation)
	for _, v := range violations {
		name := "Unknown Rule"
		if v.Rule != nil {
			name = v.Rule.Name
		}
		result[name] = append(result[name], v)
	}
	return result
}

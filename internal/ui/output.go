package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// PrintSuccess prints a green success message: ✔ msg
func PrintSuccess(msg string) {
	icon := lipgloss.NewStyle().Foreground(ColorSuccess).Render(IconOK.Emoji)
	fmt.Printf("%s %s\n", icon, msg)
}

// PrintWarning prints a yellow warning message: ⚠ msg
func PrintWarning(msg string) {
	icon := lipgloss.NewStyle().Foreground(ColorWarning).Render(IconWarn.Emoji)
	fmt.Printf("%s %s\n", icon, msg)
}

// PrintError prints a red error message: ✖ msg
func PrintError(msg string) {
	icon := lipgloss.NewStyle().Foreground(ColorError).Render(IconFail.Emoji)
	fmt.Printf("%s %s\n", icon, msg)
}

// PrintHint prints a dim hint message: 💡 msg
func PrintHint(msg string) {
	style := lipgloss.NewStyle().Foreground(ColorDim)
	fmt.Printf("%s %s\n", IconHint.Emoji, style.Render(msg))
}

// PrintInfo prints a subtle info message: ℹ msg
func PrintInfo(msg string) {
	style := lipgloss.NewStyle().Foreground(ColorDim)
	fmt.Printf("%s %s\n", IconInfo.Emoji, style.Render(msg))
}

// PrintKeyValue prints a key-value pair with aligned formatting.
//
//	Key: value
func PrintKeyValue(key, value string) {
	keyStyle := lipgloss.NewStyle().Foreground(ColorDim)
	fmt.Printf("   %s %s\n", keyStyle.Render(key+":"), value)
}

// PrintSectionHeader prints a styled section header with an icon.
//
//	\n📊 Title
func PrintSectionHeader(icon Icon, title string) {
	style := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(ColorText)
	fmt.Printf("\n%s %s\n", icon.Emoji, style.Render(title))
}

// PrintDivider prints a subtle horizontal divider line.
func PrintDivider() {
	style := lipgloss.NewStyle().Foreground(ColorDim)
	fmt.Println(style.Render(strings.Repeat("─", 40)))
}

// BootstrapStats holds accumulated stats for the final summary panel.
type BootstrapStats struct {
	FilesScanned      int
	SymbolsFound      int
	CallRelationships int
	MetadataItems     int
	AnalysisFindings  int
	AnalysisRelations int
	TotalDuration     time.Duration
}

// PrintPhaseHeader prints a numbered phase with icon, title, and description.
//
//	[1/3] 🔍 Indexing Code Symbols
//	      Scanning source files for functions, types, and call relationships.
func PrintPhaseHeader(step, total int, icon Icon, title, description string) {
	stepStyle := lipgloss.NewStyle().Foreground(ColorDim)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	descStyle := lipgloss.NewStyle().Foreground(ColorDim)

	fmt.Printf("\n  %s %s %s\n",
		stepStyle.Render(fmt.Sprintf("[%d/%d]", step, total)),
		icon.Emoji,
		titleStyle.Render(title))
	if description != "" {
		// Indent description to align with title text
		fmt.Printf("        %s\n", descStyle.Render(description))
	}
	fmt.Println()
}

// PrintPhaseResult prints a check mark + message + right-aligned dim duration.
//
//	✔ Indexed 12 updates                                   1.23s
func PrintPhaseResult(msg string, duration time.Duration) {
	icon := lipgloss.NewStyle().Foreground(ColorSuccess).Render(IconOK.Emoji)
	durStyle := lipgloss.NewStyle().Foreground(ColorDim)
	durStr := FormatDuration(duration)

	w := ContentWidth()
	// 8 chars for leading indent, 2 for icon+space
	msgWidth := w - 10 - len(durStr) - 1
	if msgWidth < 20 {
		msgWidth = 20
	}
	padded := msg
	if len(padded) < msgWidth {
		padded = padded + strings.Repeat(" ", msgWidth-len(padded))
	}

	fmt.Printf("        %s %s %s\n", icon, padded, durStyle.Render(durStr))
}

// PrintPhaseDetail prints an indented detail line (no icon).
func PrintPhaseDetail(msg string) {
	fmt.Printf("        %s\n", msg)
}

// PrintPhaseSeparator prints a dim heavy horizontal rule at ContentWidth.
func PrintPhaseSeparator() {
	style := lipgloss.NewStyle().Foreground(ColorDim)
	fmt.Println()
	fmt.Printf("  %s\n", style.Render(strings.Repeat("━", ContentWidth()-4)))
}

// RenderBootstrapWelcome returns the welcome panel string.
func RenderBootstrapWelcome() string {
	content := "Analyzes your codebase to build persistent architectural\n" +
		"memory for AI assistants — indexing symbols, extracting\n" +
		"metadata, and discovering patterns and decisions."
	return RenderInfoPanel(fmt.Sprintf("%s TaskWing Bootstrap", IconRobot.Emoji), content)
}

// RenderBootstrapSummary returns the final summary panel string.
func RenderBootstrapSummary(stats BootstrapStats) string {
	var lines []string

	if stats.FilesScanned > 0 || stats.SymbolsFound > 0 {
		line := fmt.Sprintf("  Symbols    %d files, %d symbols", stats.FilesScanned, stats.SymbolsFound)
		if stats.CallRelationships > 0 {
			line += fmt.Sprintf(", %d relationships", stats.CallRelationships)
		}
		lines = append(lines, line)
	}
	if stats.MetadataItems > 0 {
		lines = append(lines, fmt.Sprintf("  Metadata   %d items", stats.MetadataItems))
	}
	if stats.AnalysisFindings > 0 {
		line := fmt.Sprintf("  Analysis   %d findings", stats.AnalysisFindings)
		if stats.AnalysisRelations > 0 {
			line += fmt.Sprintf(", %d relationships", stats.AnalysisRelations)
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %s Run `taskwing ask \"your question\"` to query knowledge", IconHint.Emoji))

	title := fmt.Sprintf("%s Bootstrap Complete", IconOK.Emoji)
	durStr := FormatDuration(stats.TotalDuration)
	title += strings.Repeat(" ", max(0, ContentWidth()-len(title)-len(durStr)-6)) + durStr

	return RenderSuccessPanel(title, strings.Join(lines, "\n"))
}

// RenderPlanBox returns a numbered action list inside a bordered box.
func RenderPlanBox(actions []string) string {
	var lines []string
	for i, action := range actions {
		lines = append(lines, fmt.Sprintf(" %d. %s", i+1, action))
	}
	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(0, 1).
		Width(ContentWidth())

	titleStyle := lipgloss.NewStyle().Foreground(ColorDim)
	return "\n" + style.Render(titleStyle.Render("Plan")+"\n"+content) + "\n"
}

// formatDuration formats a duration for display: "1.23s", "2m12s", etc.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", mins, secs)
}


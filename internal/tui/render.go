package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// truncate shortens s to maxLen runes, adding an ellipsis if needed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return ellipsis
	}
	return string(runes[:maxLen-1]) + ellipsis
}

// padRight pads s with spaces on the right to reach width.
func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// centerInScreen returns content padded with spaces and blank lines so that
// it appears centred within a terminal of size (screenW, screenH).
func centerInScreen(content string, screenW, screenH int) string {
	lines := strings.Split(content, "\n")
	h := len(lines)
	w := lipgloss.Width(content)

	topPad := (screenH - h) / 2
	if topPad < 0 {
		topPad = 0
	}
	leftPad := (screenW - w) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	leftStr := strings.Repeat(" ", leftPad)

	var sb strings.Builder
	for i := 0; i < topPad; i++ {
		sb.WriteString("\n")
	}
	for _, l := range lines {
		sb.WriteString(leftStr + l + "\n")
	}
	return sb.String()
}

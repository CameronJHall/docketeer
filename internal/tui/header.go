package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/CameronJHall/docketeer/internal/task"
)

// sparklineChars are Unicode block elements from shortest to tallest.
var sparklineChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// HeaderView renders the app header bar.
func RenderHeader(width int, items []task.Item, completions []int, showMetrics bool) string {
	// Title
	title := StyleAccent.Render("docketeer")

	// Counts
	var taskCount, ideaCount, inProgressCount int
	for _, item := range items {
		if item.IsIdea() {
			ideaCount++
		} else {
			taskCount++
			if item.Status != nil && *item.Status == task.StatusInProgress {
				inProgressCount++
			}
		}
	}
	counts := StyleMuted.Render(fmt.Sprintf("%d tasks · %d ideas", taskCount, ideaCount))
	if inProgressCount > 0 {
		counts += " " + fg(colorActiveCount).Render(fmt.Sprintf("(%d active)", inProgressCount))
	}

	var metricsParts []string

	if showMetrics {
		// Sparkline
		if completions != nil {
			sparkline := renderSparkline(completions)
			if sparkline != "" {
				metricsParts = append(metricsParts, StyleMuted.Render("7d: ")+sparkline)
			}
		}

		// Backlog age
		age := task.BacklogAge(items)
		if age > 0 {
			ageStr := fmt.Sprintf("debt: %dd", age)
			var ageStyle lipgloss.Style
			switch {
			case age >= 100:
				ageStyle = StyleAgeCritical
			case age >= 50:
				ageStyle = StyleAgeHigh
			case age >= 20:
				ageStyle = StyleAgeWarning
			default:
				ageStyle = StyleAgeHealthy
			}
			metricsParts = append(metricsParts, ageStyle.Render(ageStr))
		}
	}

	// Compose right side
	right := counts
	if len(metricsParts) > 0 {
		right += "  " + strings.Join(metricsParts, "  ")
	}

	// Pad between title and right side
	titleWidth := lipgloss.Width(title)
	rightWidth := lipgloss.Width(right)
	gap := width - titleWidth - rightWidth - 2
	if gap < 1 {
		gap = 1
	}
	line := title + strings.Repeat(" ", gap) + right

	return lipgloss.NewStyle().
		Width(width).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorDivider).
		Render(line)
}

// renderSparkline builds a 7-char sparkline string from daily counts.
func renderSparkline(counts []int) string {
	if len(counts) == 0 {
		return ""
	}
	// Find peak
	peak := 0
	allZero := true
	for _, c := range counts {
		if c > peak {
			peak = c
		}
		if c > 0 {
			allZero = false
		}
	}
	if allZero {
		return ""
	}

	var sb strings.Builder
	for _, c := range counts {
		var idx int
		if peak > 0 {
			idx = (c * (len(sparklineChars) - 1)) / peak
		}
		bar := sparklineChars[idx]
		if c > 0 {
			sb.WriteString(fg(colorSparkline).Render(bar))
		} else {
			sb.WriteString(StyleMuted.Render(bar))
		}
	}
	return sb.String()
}

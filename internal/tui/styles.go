package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/CameronJHall/docketeer/internal/task"
)

// Accent / branding
var (
	colorAccent  = lipgloss.Color("63")  // blue-purple
	colorMuted   = lipgloss.Color("240") // dim gray
	colorDivider = lipgloss.Color("237") // dark gray (unfocused)
)

// Priority colors: cool (low) → warm (critical)
var priorityColors = map[task.Priority]color.Color{
	task.PriorityLow:      lipgloss.Color("38"),  // steel blue
	task.PriorityMedium:   lipgloss.Color("71"),  // muted green
	task.PriorityHigh:     lipgloss.Color("214"), // orange
	task.PriorityCritical: lipgloss.Color("167"), // salmon
}

// Status dot colors
var statusColors = map[task.Status]color.Color{
	task.StatusTodo:       lipgloss.Color("245"), // gray
	task.StatusInProgress: lipgloss.Color("33"),  // blue
	task.StatusDone:       lipgloss.Color("35"),  // green
	task.StatusBlocked:    lipgloss.Color("174"), // light rose
}

// Status dot characters
var statusDots = map[task.Status]string{
	task.StatusTodo:       "○",
	task.StatusInProgress: "●",
	task.StatusDone:       "✓",
	task.StatusBlocked:    "✗",
}

// Idea accent
var colorIdea = lipgloss.Color("183") // lavender

// Decay colors
var decayColors = map[task.DecayLevel]color.Color{
	task.DecaySubtle:   lipgloss.Color("242"),
	task.DecayModerate: lipgloss.Color("214"),
	task.DecayWarning:  lipgloss.Color("208"),
	task.DecayAlert:    lipgloss.Color("167"),
}

// 16 dusty pastel project colors (deterministic hash palette)
var projectPalette = []color.Color{
	lipgloss.Color("67"),  // steel blue
	lipgloss.Color("71"),  // sage green
	lipgloss.Color("136"), // gold
	lipgloss.Color("131"), // dusty rose
	lipgloss.Color("97"),  // lavender
	lipgloss.Color("73"),  // teal
	lipgloss.Color("179"), // warm yellow
	lipgloss.Color("167"), // salmon
	lipgloss.Color("110"), // periwinkle
	lipgloss.Color("107"), // olive
	lipgloss.Color("173"), // peach
	lipgloss.Color("139"), // mauve
	lipgloss.Color("80"),  // aqua
	lipgloss.Color("143"), // yellow-green
	lipgloss.Color("174"), // light rose
	lipgloss.Color("116"), // pale teal
}

// ProjectColor returns a deterministic color for a project name.
func ProjectColor(project string) color.Color {
	if project == "" {
		return colorMuted
	}
	h := 0
	for _, c := range project {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return projectPalette[h%len(projectPalette)]
}

// PriorityColor returns the color for a priority level.
func PriorityColor(p task.Priority) color.Color {
	if c, ok := priorityColors[p]; ok {
		return c
	}
	return colorMuted
}

// StatusColor returns the color for a status.
func StatusColor(s task.Status) color.Color {
	if c, ok := statusColors[s]; ok {
		return c
	}
	return colorMuted
}

// StatusDot returns the dot character for a status.
func StatusDot(s task.Status) string {
	if d, ok := statusDots[s]; ok {
		return d
	}
	return "·"
}

// DecayColor returns the color for a decay level.
func DecayColor(d task.DecayLevel) color.Color {
	if c, ok := decayColors[d]; ok {
		return c
	}
	return colorMuted
}

// fg returns a plain foreground-only style for the given color.
// Use this instead of lipgloss.NewStyle().Foreground(c) at call sites.
func fg(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

// Fixed colors used outside the color-lookup maps.
var (
	colorActiveCount = lipgloss.Color("33")  // blue  — "N active" in header
	colorSparkline   = lipgloss.Color("35")  // green — sparkline filled bars
	colorDecayDim    = lipgloss.Color("240") // gray  — dimmed title text
	colorError       = lipgloss.Color("167") // salmon — validation errors
	colorConfirmBox  = lipgloss.Color("167") // salmon — delete confirm border
	colorConfirmText = lipgloss.Color("252") // near-white — confirm box text
)

// Backlog age threshold styles (used in header).
var (
	StyleAgeHealthy  = fg(lipgloss.Color("35"))
	StyleAgeWarning  = fg(lipgloss.Color("214"))
	StyleAgeHigh     = fg(lipgloss.Color("208"))
	StyleAgeCritical = lipgloss.NewStyle().Foreground(lipgloss.Color("167")).Bold(true)
)

// --- Shared styles ---

var (
	StyleAccent = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	StyleMuted  = lipgloss.NewStyle().Foreground(colorMuted)
	StyleBold   = lipgloss.NewStyle().Bold(true)

	// Panel layout
	StyleLeftPanel = lipgloss.NewStyle().
			PaddingRight(1)

	StyleRightPanel = lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorDivider)

	StyleRightPanelFocused = lipgloss.NewStyle().
				PaddingLeft(1).
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorAccent)

	// Group header
	StyleGroupHeader = lipgloss.NewStyle().
				Foreground(colorMuted).
				Bold(true)

	// Selected item highlight
	StyleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Bold(true)

	// Idea line
	StyleIdea = lipgloss.NewStyle().Foreground(colorIdea)

	// Detail panel
	StyleDetailTitle = lipgloss.NewStyle().Bold(true)
	StyleDetailMeta  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	StyleDetailNote  = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))

	// Status message
	StyleStatusMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("35")).Italic(true)

	// Overdue
	StyleOverdue = lipgloss.NewStyle().Foreground(lipgloss.Color("167"))

	// Form
	StyleFormError        = fg(colorError)
	StyleFormFocusedLabel = fg(colorAccent)

	// Calendar active state — left border in accent colour to signal editing mode.
	StyleCalendarActive = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorAccent).
				PaddingLeft(1)

	// Confirm overlay
	StyleConfirmBox = lipgloss.NewStyle().
			Padding(1, 3).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorConfirmBox).
			Foreground(colorConfirmText)
)

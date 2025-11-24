package telemetry

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/telemetry"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
)

// Snapshot captures a point-in-time view of telemetry aggregates.
type Snapshot struct {
	Events     []telemetry.Event
	TokensIn   int64
	TokensOut  int64
	Cost       float64
	ToolTotals []ToolTotal
	SessionID  string
}

// ToolTotal captures aggregate stats per tool.
type ToolTotal struct {
	Name      string
	Calls     int
	TokensIn  int64
	TokensOut int64
	Cost      float64
}

// UpdateMsg refreshes the dialog with new telemetry data.
type UpdateMsg struct {
	Snapshot Snapshot
}

const TelemetryDialogID dialogs.DialogID = "telemetry"

type telemetryDialogCmp struct {
	wWidth, wHeight int
	width, height   int

	viewport      viewport.Model
	snapshot      Snapshot
	positionRow   int
	positionCol   int
	finalHeight   int
	filterSession bool
	scopeHint     string
	filterTool    string
	toolCycle     []string
	toolIndex     int
}

// NewTelemetryDialogCmp creates a read-only dialog showing recent telemetry.
func NewTelemetryDialogCmp(snapshot Snapshot) dialogs.DialogModel {
	vp := viewport.New()
	return &telemetryDialogCmp{
		viewport: vp,
		snapshot: snapshot,
	}
}

func (d *telemetryDialogCmp) Init() tea.Cmd {
	return d.viewport.Init()
}

func (d *telemetryDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth, d.wHeight = msg.Width, msg.Height
		d.SetSize()
		return d, nil
	case UpdateMsg:
		d.snapshot = msg.Snapshot
		d.refreshToolCycle()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "s", "S":
			if d.snapshot.SessionID != "" {
				d.filterSession = !d.filterSession
			}
		case "t", "T":
			if len(d.toolCycle) > 0 {
				d.toolIndex = (d.toolIndex + 1) % (len(d.toolCycle) + 1)
				if d.toolIndex == 0 {
					d.filterTool = ""
				} else {
					d.filterTool = d.toolCycle[d.toolIndex-1]
				}
			}
		case "q", "esc":
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}

	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

func (d *telemetryDialogCmp) renderContent() string {
	t := styles.CurrentTheme()
	width := d.viewport.Width()
	if width <= 0 {
		width = max(20, d.width-4)
	}

	events := d.snapshot.Events
	if d.filterSession && d.snapshot.SessionID != "" {
		filtered := events[:0]
		for _, ev := range events {
			if ev.SessionID == d.snapshot.SessionID {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}
	if d.filterTool != "" {
		filtered := events[:0]
		for _, ev := range events {
			name := ev.ToolName
			if name == "" {
				name = string(ev.Type)
			}
			if name == d.filterTool {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	if len(events) == 0 {
		return t.S().Base.Background(t.BgSubtle).
			Width(width).
			Padding(1, 2).
			Render(t.S().Muted.Render("No telemetry yet. Trigger a tool to see live usage."))
	}

	summary := fmt.Sprintf("Tokens in/out: %d / %d • Cost: $%.4f", d.snapshot.TokensIn, d.snapshot.TokensOut, d.snapshot.Cost)
	lines := []string{
		t.S().Text.Bold(true).Render(ansi.Truncate(summary, width, "…")),
	}

	if len(d.snapshot.ToolTotals) > 0 {
		lines = append(lines, t.S().Muted.Render("Top tools (by cost):"))
		for i, tt := range d.snapshot.ToolTotals {
			if i >= 5 {
				break
			}
			line := fmt.Sprintf("  %-14s calls:%d in:%d out:%d cost:%.4f",
				ansi.Truncate(tt.Name, 14, "…"),
				tt.Calls,
				tt.TokensIn,
				tt.TokensOut,
				tt.Cost,
			)
			lines = append(lines, ansi.Truncate(line, width, "…"))
		}
	}

	lines = append(lines, t.S().Muted.Render(strings.Repeat("─", width)))

	maxEvents := min(len(events), 20)
	for i := len(events) - 1; i >= len(events)-maxEvents; i-- {
		ev := events[i]
		tool := ev.ToolName
		if tool == "" {
			tool = string(ev.Type)
		}
		stamp := ev.At.Format("15:04:05")
		duration := ""
		if ev.Duration > 0 {
			duration = ev.Duration.Truncate(10 * time.Millisecond).String()
		}
		line := fmt.Sprintf(
			"%s  %-14s %-12s in:%d out:%d cost:%.4f %s",
			stamp,
			ansi.Truncate(tool, 14, "…"),
			ev.Type,
			ev.TokensIn,
			ev.TokensOut,
			ev.Cost,
			duration,
		)
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}

	return t.S().Base.Background(t.BgSubtle).
		Width(width).
		Padding(1, 1).
		Render(strings.Join(lines, "\n"))
}

func (d *telemetryDialogCmp) View() string {
	t := styles.CurrentTheme()
	title := core.Title("Telemetry", d.width-4)
	if d.snapshot.SessionID != "" {
		scope := "All sessions"
		if d.filterSession {
			scope = "Session-only"
		}
		tool := "all tools"
		if d.filterTool != "" {
			tool = d.filterTool
		}
		d.scopeHint = t.S().Muted.Render(fmt.Sprintf("Scope: %s • Tool: %s", scope, tool))
	} else {
		tool := "all tools"
		if d.filterTool != "" {
			tool = d.filterTool
		}
		d.scopeHint = t.S().Muted.Render("Tool: " + tool)
	}

	d.viewport.SetWidth(d.width - 4)
	d.viewport.SetHeight(max(8, d.height-6))
	content := d.renderContent()
	contentHeight := min(d.viewport.Height(), lipgloss.Height(content))
	d.viewport.SetHeight(contentHeight)
	d.viewport.SetContent(content)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		d.scopeHint,
		"",
		d.viewport.View(),
		"",
		t.S().Muted.Render("↑/↓ scroll • s filter session • t cycle tool • esc to close"),
	)

	dialog := t.S().Base.
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(d.width).
		Render(body)

	d.finalHeight = lipgloss.Height(dialog)
	d.positionRow = d.wHeight/2 - d.finalHeight/2
	d.positionCol = d.wWidth/2 - d.width/2

	return dialog
}

func (d *telemetryDialogCmp) Position() (int, int) {
	return d.positionRow, d.positionCol
}

func (d *telemetryDialogCmp) ID() dialogs.DialogID {
	return TelemetryDialogID
}

func (d *telemetryDialogCmp) SetSize() {
	d.width = int(float64(d.wWidth) * 0.7)
	d.height = int(float64(d.wHeight) * 0.6)
	d.width = min(d.width, 120)
}

func (d *telemetryDialogCmp) refreshToolCycle() {
	if len(d.snapshot.ToolTotals) == 0 {
		d.toolCycle = nil
		d.filterTool = ""
		d.toolIndex = 0
		return
	}
	names := make([]string, 0, len(d.snapshot.ToolTotals))
	for _, tt := range d.snapshot.ToolTotals {
		names = append(names, tt.Name)
	}
	d.toolCycle = names
	if d.filterTool == "" {
		d.toolIndex = 0
		return
	}
	for i, name := range names {
		if name == d.filterTool {
			d.toolIndex = i + 1
			return
		}
	}
	d.toolIndex = 0
	d.filterTool = ""
}

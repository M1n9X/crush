package freshness

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/freshness"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
)

const DialogID dialogs.DialogID = "freshness"

// NewFreshnessDialogCmp renders a list of freshest files for the session.
func NewFreshnessDialogCmp(items []freshness.Meta) dialogs.DialogModel {
	vp := viewport.New()
	return &dialog{
		vp:    vp,
		items: items,
	}
}

type dialog struct {
	wWidth, wHeight int
	width, height   int
	vp              viewport.Model
	items           []freshness.Meta
	positionRow     int
	positionCol     int
}

func (d *dialog) Init() tea.Cmd {
	return d.vp.Init()
}

func (d *dialog) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth, d.wHeight = msg.Width, msg.Height
		d.setSize()
	case UpdateMsg:
		d.items = msg.Items
	}
	var cmd tea.Cmd
	d.vp, cmd = d.vp.Update(msg)
	return d, cmd
}

// UpdateMsg refreshes the list.
type UpdateMsg struct {
	Items []freshness.Meta
}

func (d *dialog) renderContent() string {
	t := styles.CurrentTheme()
	width := d.vp.Width()
	if width <= 0 {
		width = max(20, d.width-4)
	}
	if len(d.items) == 0 {
		return t.S().Muted.Render("No recent files for this session yet.")
	}

	lines := []string{
		t.S().Muted.Render("Freshest files by recent access:"),
		t.S().Muted.Render(strings.Repeat("─", width)),
	}
	for _, meta := range d.items {
		ago := time.Since(meta.LastAccess).Truncate(time.Second)
		line := fmt.Sprintf("• %s (hits:%d, last:%s ago)", ansi.Truncate(meta.Path, width-20, "…"), meta.Count, ago)
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	return strings.Join(lines, "\n")
}

func (d *dialog) View() string {
	t := styles.CurrentTheme()
	title := core.Title("Fresh Context", d.width-4)

	d.vp.SetWidth(d.width - 4)
	d.vp.SetHeight(max(8, d.height-6))
	content := d.renderContent()
	contentHeight := min(d.vp.Height(), lipgloss.Height(content))
	d.vp.SetHeight(contentHeight)
	d.vp.SetContent(content)

	body := lipgloss.JoinVertical(lipgloss.Left, title, "", d.vp.View())
	dialog := t.S().Base.
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(d.width).
		Render(body)

	d.positionRow = d.wHeight/2 - lipgloss.Height(dialog)/2
	d.positionCol = d.wWidth/2 - d.width/2
	return dialog
}

func (d *dialog) Position() (int, int) {
	return d.positionRow, d.positionCol
}

func (d *dialog) ID() dialogs.DialogID {
	return DialogID
}

func (d *dialog) setSize() {
	d.width = min(int(float64(d.wWidth)*0.7), 120)
	d.height = int(float64(d.wHeight) * 0.6)
}

package plugins

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
)

const DialogID dialogs.DialogID = "plugins"

// TogglePluginMsg is emitted when the user toggles a plugin.
type TogglePluginMsg struct {
	Name   string
	Enable bool
}

// RefreshMsg refreshes the list of plugins.
type RefreshMsg struct {
	Plugins []plugin.Descriptor
}

// NewDialog builds a plugin manager dialog.
func NewDialog(descs []plugin.Descriptor) dialogs.DialogModel {
	vp := viewport.New()
	return &dialog{
		vp:      vp,
		plugins: descs,
	}
}

type dialog struct {
	wWidth, wHeight int
	width, height   int
	vp              viewport.Model
	plugins         []plugin.Descriptor
	selected        int
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
		return d, nil
	case RefreshMsg:
		d.plugins = msg.Plugins
		if d.selected >= len(d.plugins) {
			d.selected = max(0, len(d.plugins)-1)
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
			if len(d.plugins) == 0 {
				return d, nil
			}
			d.selected = (d.selected - 1 + len(d.plugins)) % len(d.plugins)
		case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
			if len(d.plugins) == 0 {
				return d, nil
			}
			d.selected = (d.selected + 1) % len(d.plugins)
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))),
			key.Matches(msg, key.NewBinding(key.WithKeys("right"))),
			key.Matches(msg, key.NewBinding(key.WithKeys("left"))):
			if len(d.plugins) == 0 {
				return d, nil
			}
			p := d.plugins[d.selected]
			p.Enabled = !p.Enabled
			d.plugins[d.selected] = p
			return d, util.CmdHandler(TogglePluginMsg{Name: p.Name, Enable: p.Enabled})
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc"))):
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	var cmd tea.Cmd
	d.vp, cmd = d.vp.Update(msg)
	return d, cmd
}

func (d *dialog) renderContent() string {
	t := styles.CurrentTheme()
	width := d.vp.Width()
	if width <= 0 {
		width = max(20, d.width-4)
	}

	if len(d.plugins) == 0 {
		return t.S().Muted.Render("No plugins registered.")
	}

	var lines []string
	header := fmt.Sprintf("%-18s %-8s %-8s %-10s %s", "Name", "API", "State", "Sandbox", "Path")
	lines = append(lines, t.S().Muted.Render(ansi.Truncate(header, width, "…")))
	lines = append(lines, t.S().Muted.Render(strings.Repeat("─", width)))

	for i, p := range d.plugins {
		state := "enabled"
		if !p.Enabled {
			state = "disabled"
		}
		line := fmt.Sprintf("%-18s %-8s %-8s %-10s %s",
			ansi.Truncate(p.Name, 18, "…"),
			ansi.Truncate(p.APIVersion, 8, "…"),
			state,
			ansi.Truncate(p.Sandbox, 10, "…"),
			ansi.Truncate(p.Path, max(8, width-50), "…"),
		)
		if i == d.selected {
			line = t.S().TextSelected.Render(line)
		} else {
			line = t.S().Text.Render(line)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (d *dialog) View() string {
	t := styles.CurrentTheme()
	title := core.Title("Plugins", d.width-4)

	d.vp.SetWidth(d.width - 4)
	d.vp.SetHeight(max(8, d.height-6))
	content := d.renderContent()
	d.vp.SetContent(content)

	body := strings.Join([]string{
		title,
		"",
		d.vp.View(),
		"",
		t.S().Muted.Render("↑/↓ to select, enter/space to toggle, esc to close"),
	}, "\n")

	dialog := t.S().Base.
		Padding(0, 1).
		BorderForeground(t.BorderFocus).
		Border(lipgloss.RoundedBorder()).
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

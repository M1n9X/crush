package hyper

import (
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/tui/util"
)

// Stub implementations to satisfy interfaces when Hyper is not available.
type DeviceAuthInitiatedMsg struct{}
type DeviceFlowErrorMsg struct{ Err error }
type DeviceFlowCompletedMsg struct {
	Token string
}

type DeviceFlow struct{}

func NewDeviceFlow() *DeviceFlow {
	return &DeviceFlow{}
}

func (d *DeviceFlow) SetWidth(int) {}

func (d *DeviceFlow) Init() tea.Cmd {
	return nil
}

func (d *DeviceFlow) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	return d, nil
}

func (d *DeviceFlow) CopyCode() tea.Cmd {
	return nil
}

func (d *DeviceFlow) CopyCodeAndOpenURL() tea.Cmd {
	return nil
}

func (d *DeviceFlow) Cancel() {}

func (d *DeviceFlow) View() string {
	return ""
}

func (d *DeviceFlow) Cursor() *tea.Cursor {
	return nil
}

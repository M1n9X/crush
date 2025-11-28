package editor

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// promptHistory tracks previously submitted prompts and supports shell-style
// navigation with Up/Down.
type promptHistory struct {
	manager   *HistoryManager
	project   string
	entries   []string
	cursor    int
	lastValue string
}

func newPromptHistory(project string) promptHistory {
	manager := NewHistoryManager()
	entries, _ := manager.GetProjectHistory(project)

	return promptHistory{
		manager: manager,
		project: project,
		entries: entries,
		cursor:  -1,
	}
}

func (h *promptHistory) clear() {
	h.entries = nil
	h.resetNavigation()
}

func (h *promptHistory) resetNavigation() {
	h.cursor = -1
	h.lastValue = ""
}

func (h *promptHistory) setEntries(entries []string) {
	h.entries = sanitizeEntries(entries)
	h.resetNavigation()
}

func (h *promptHistory) record(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}

	// Save to persistent storage
	if h.manager != nil {
		_ = h.manager.Save(entry, h.project)
	}

	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == entry {
		h.resetNavigation()
		return
	}
	h.entries = append(h.entries, entry)
	h.resetNavigation()
}

func (h *promptHistory) shouldHandleNavigation(text string, cursor *tea.Cursor) bool {
	if len(h.entries) == 0 || cursor == nil {
		return false
	}
	if strings.TrimSpace(text) == "" {
		return true
	}
	// Allow navigation when cursor is on the first line (Y==0)
	// This matches standard shell behavior: Up/Down navigate history on first line,
	// but move cursor on other lines in multi-line input
	if cursor.Y == 0 {
		return true
	}
	return false
}

func (h *promptHistory) previous() (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}

	switch h.cursor {
	case -1:
		h.cursor = len(h.entries) - 1
	case 0:
		return "", false
	default:
		h.cursor--
	}

	h.lastValue = h.entries[h.cursor]
	return h.lastValue, true
}

func (h *promptHistory) next() (string, bool) {
	if len(h.entries) == 0 || h.cursor == -1 {
		return "", false
	}

	if h.cursor >= len(h.entries)-1 {
		h.resetNavigation()
		return "", true
	}

	h.cursor++
	h.lastValue = h.entries[h.cursor]
	return h.lastValue, true
}

func sanitizeEntries(entries []string) []string {
	clean := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if len(clean) > 0 && clean[len(clean)-1] == entry {
			continue
		}
		clean = append(clean, entry)
	}
	return clean
}

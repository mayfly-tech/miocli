package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/profile"
	"miocli/internal/theme"
)

func WithProfiles(service *profile.Service) Option {
	return func(m *Model) { m.profileService = service }
}

type profilesMsg struct {
	index      profile.Index
	listErr    string
	err        error
	operation  bool
	pending    bool
	generation uint64
}

func (m *Model) readProfiles() profilesMsg {
	if m.profileService == nil {
		return profilesMsg{listErr: "Full profiles require an owned managed core; press p for Providers"}
	}
	index, err := m.profileService.List()
	message := profilesMsg{index: index}
	if err != nil {
		message.listErr = err.Error()
	}
	pending, pendingErr := m.profileService.Pending()
	message.pending = pending
	if pendingErr != nil {
		message.listErr = pendingErr.Error()
	}
	return message
}

func (m *Model) loadProfiles() tea.Cmd {
	generation := m.generation
	return func() tea.Msg {
		message := m.readProfiles()
		message.generation = generation
		return message
	}
}

func (m *Model) profileRows() []resourceRow {
	rows := make([]resourceRow, 0, len(m.profileIndex.Entries))
	for _, entry := range m.profileIndex.Entries {
		active := "Inactive"
		if entry.ID == m.profileIndex.Active {
			active = "Active"
		}
		source := "HTTPS"
		if entry.Kind == "file" {
			source = "Local YAML"
		}
		rows = append(rows, resourceRow{entry.ID, []string{entry.Name, source, active, entry.Updated.Local().Format("01-02 15:04")}, []string{"Name: " + entry.Name, "Source: " + source + " (address hidden)", "State: " + unknown(active), "Updated: " + entry.Updated.Local().Format("2006-01-02 15:04:05"), "One full profile active at a time; no merging", "Changing an active profile requires confirmed hot reload", "Only copies are changed; sources stay read-only"}})
	}
	return rows
}

func (m *Model) profileEmptyText() string {
	if m.profileErr != "" {
		return m.profileErr
	}
	if !m.runtime.Owned {
		return "Full profiles require an owned managed core. Press p for Providers"
	}
	return "No profiles yet. Press a to add local YAML or an HTTPS subscription"
}

type profileDialog struct {
	action     string
	id         string
	fields     []textinput.Model
	field      int
	confirming bool
	yes        bool
	warning    string
	err        string
}

func (m *Model) profileKey(key string) (bool, tea.Cmd) {
	if !m.runtime.Owned || m.profileService == nil {
		m.lastError = "Full profiles require an owned managed core; press p for Providers"
		return true, nil
	}
	if m.operation != "" || m.profilePending || m.profileErr != "" {
		m.lastError = "Profile writes blocked; resolve the operation, profile error or recovery first"
		return true, nil
	}
	dialog := &profileDialog{}
	switch key {
	case "a":
		dialog.action = "add"
	case "e", "d", "u", " ":
		rows := m.resourceRows()
		if len(rows) == 0 {
			m.notice = "Add a profile first"
			return true, nil
		}
		id := rows[m.resourceIndex(rows)].id
		dialog.id = id
		switch key {
		case "e":
			dialog.action = "edit"
		case "d":
			dialog.action = "delete"
		case "u":
			dialog.action = "refresh"
		case " ":
			dialog.action = "apply"
		}
		if dialog.action == "delete" && id == m.profileIndex.Active {
			m.lastError = "Cannot delete the active profile; activate another first"
			return true, nil
		}
	default:
		return false, nil
	}
	if dialog.action == "add" || dialog.action == "edit" {
		values := []string{"", "https", ""}
		for _, entry := range m.profileIndex.Entries {
			if entry.ID == dialog.id {
				values = []string{entry.Name, entry.Kind, entry.Source}
				break
			}
		}
		for i, value := range values {
			input := theme.Input(m.noColor)
			input.Width = max(10, m.width-8)
			input.CharLimit = 4096
			input.SetValue(value)
			if i == 2 {
				input.EchoMode = textinput.EchoPassword
				input.EchoCharacter = '*'
			}
			dialog.fields = append(dialog.fields, input)
		}
		dialog.fields[0].CharLimit = 128
		dialog.fields[1].CharLimit = 5
		dialog.fields[0].Focus()
	} else {
		dialog.confirming = true
	}
	if dialog.id == m.profileIndex.Active && (dialog.action == "edit" || dialog.action == "refresh") || dialog.action == "apply" {
		dialog.warning = "This validates and hot reloads the owned core. Existing connections may be affected. No automatic restart."
	}
	m.lastError = ""
	m.dialog = dialog
	return true, nil
}

func (m *Model) dialogKey(key tea.KeyMsg) tea.Cmd {
	dialog := m.dialog
	if key.Type == tea.KeyEsc {
		m.dialog = nil
		return nil
	}
	if dialog.confirming {
		switch key.String() {
		case "tab", "left", "right":
			dialog.yes = !dialog.yes
		case "n":
			m.dialog = nil
		case "y":
			return m.runProfileDialog()
		case "enter":
			if dialog.yes {
				return m.runProfileDialog()
			}
			m.dialog = nil
		}
		return nil
	}
	if dialog.field == 1 {
		switch key.String() {
		case "left", "right", "up", "down", "h", "l", "j", "k":
			kind := "https"
			if dialog.fields[1].Value() == kind {
				kind = "file"
			}
			dialog.fields[1].SetValue(kind)
			return nil
		}
	}
	switch key.Type {
	case tea.KeyTab, tea.KeyEnter, tea.KeyShiftTab:
		dialog.fields[dialog.field].Blur()
		if key.Type == tea.KeyEnter && dialog.field == len(dialog.fields)-1 {
			if strings.TrimSpace(dialog.fields[0].Value()) == "" {
				dialog.field = 0
				dialog.err = "Enter a profile name"
				return dialog.fields[0].Focus()
			}
			if strings.TrimSpace(dialog.fields[2].Value()) == "" {
				dialog.err = "Enter the source path or HTTPS URL"
				dialog.fields[dialog.field].Focus()
				return nil
			}
			dialog.confirming = true
			dialog.err = ""
			return nil
		}
		delta := 1
		if key.Type == tea.KeyShiftTab {
			delta = len(dialog.fields) - 1
		}
		dialog.field = (dialog.field + delta) % len(dialog.fields)
		dialog.err = ""
		return dialog.fields[dialog.field].Focus()
	}
	if dialog.field == 1 {
		return nil
	}
	var cmd tea.Cmd
	dialog.fields[dialog.field], cmd = dialog.fields[dialog.field].Update(key)
	return cmd
}

func (m *Model) runProfileDialog() tea.Cmd {
	dialog, service, ctx := m.dialog, m.profileService, m.ctx
	m.dialog = nil
	m.cancelLatency()
	m.generation++
	generation := m.generation
	m.operation = "Processing profile"
	m.lastError = ""
	return func() tea.Msg {
		var err error
		switch dialog.action {
		case "add":
			err = service.Add(ctx, dialog.fields[0].Value(), dialog.fields[1].Value(), strings.TrimSpace(dialog.fields[2].Value()))
		case "edit":
			err = service.Edit(ctx, dialog.id, dialog.fields[0].Value(), dialog.fields[1].Value(), strings.TrimSpace(dialog.fields[2].Value()))
		case "delete":
			err = service.Delete(dialog.id)
		case "refresh":
			err = service.Refresh(ctx, dialog.id)
		case "apply":
			err = service.Apply(ctx, dialog.id)
		}
		message := m.readProfiles()
		message.operation, message.err = true, err
		message.generation = generation
		return message
	}
}

func (m *Model) dialogView(width, height int, styles theme.Styles) []string {
	dialog := m.dialog
	action := map[string]string{"add": "Add profile", "edit": "Edit profile", "delete": "Delete profile", "refresh": "Refresh profile", "apply": "Activate profile"}[dialog.action]
	lines := []string{styles.Title.Render(action), ""}
	var footer string
	if dialog.confirming {
		name := ""
		for _, entry := range m.profileIndex.Entries {
			if entry.ID == dialog.id {
				name = entry.Name
			}
		}
		if len(dialog.fields) > 0 {
			name = dialog.fields[0].Value()
		}
		lines = append(lines, wrappedLines([]string{dialog.warning}, width)...)
		lines = append(lines, "Profile: "+safeText(name), "Source addresses and credentials are hidden.", "")
		choice := "[Cancel]   Confirm"
		if dialog.yes {
			choice = " Cancel   [Confirm]"
		}
		lines = append(lines, choice)
		footer = "Tab choose · Enter submit · Esc cancel"
	} else {
		labels := []string{"Name", "Source type", "Source (hidden)"}
		lines = append(lines, fmt.Sprintf("%s · %d/3", labels[dialog.field], dialog.field+1))
		if dialog.field == 1 {
			choice := "[HTTPS subscription]   Local YAML"
			if dialog.fields[1].Value() == "file" {
				choice = " HTTPS subscription   [Local YAML]"
			}
			lines = append(lines, choice, "Left/Right to choose")
		} else {
			lines = append(lines, dialog.fields[dialog.field].View())
		}
		footer = "Tab/Shift+Tab field · Enter next · Esc cancel"
	}
	if dialog.err != "" {
		lines = append(lines, styles.Error.Render(dialog.err))
	}
	lines = lines[:min(len(lines), height-1)]
	return append(lines, styles.Muted.Render(footer))
}

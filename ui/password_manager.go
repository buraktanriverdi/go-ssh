package ui

import (
	"fmt"
	"go-ssh/password"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type passwordManagerModel struct {
	store           *password.PasswordStore
	masterPwd       string
	mode            string // "menu", "add", "list", "remove", "view", "change-master"
	entries         []*password.PasswordEntry
	cursor          int
	width           int
	height          int
	inputID         string
	inputDesc       string
	inputPwd        string
	inputOldPwd     string
	inputNewPwd     string
	inputConfirmPwd string
	inputField      int // 0=id, 1=desc, 2=pwd (for add), 0=old, 1=new, 2=confirm (for change-master)
	message         string
	messageType     string // "success", "error", "info"
	quitting        bool
	passwordAdded   bool
	viewingPassword string
}

func initialPasswordManagerModel(store *password.PasswordStore, masterPwd string) passwordManagerModel {
	return passwordManagerModel{
		store:     store,
		masterPwd: masterPwd,
		mode:      "menu",
		entries:   store.List(),
	}
}

func (m passwordManagerModel) Init() tea.Cmd {
	return nil
}

func (m passwordManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case "menu":
			return m.updateMenu(msg)
		case "add":
			return m.updateAdd(msg)
		case "list":
			return m.updateList(msg)
		case "remove":
			return m.updateRemove(msg)
		case "view":
			return m.updateView(msg)
		case "change-master":
			return m.updateChangeMaster(msg)
		}
	}

	return m, nil
}

func (m passwordManagerModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < 5 {
			m.cursor++
		}

	case "enter", " ":
		switch m.cursor {
		case 0: // Add password
			m.mode = "add"
			m.inputID = ""
			m.inputDesc = ""
			m.inputPwd = ""
			m.inputField = 0
			m.message = ""
		case 1: // View password
			m.mode = "view"
			m.entries = m.store.List()
			m.cursor = 0
			m.message = ""
			m.viewingPassword = ""
		case 2: // List passwords
			m.mode = "list"
			m.entries = m.store.List()
			m.cursor = 0
			m.message = ""
		case 3: // Remove password
			m.mode = "remove"
			m.entries = m.store.List()
			m.cursor = 0
			m.message = ""
		case 4: // Change master password
			m.mode = "change-master"
			m.inputOldPwd = ""
			m.inputNewPwd = ""
			m.inputConfirmPwd = ""
			m.inputField = 0
			m.message = ""
		case 5: // Exit
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m passwordManagerModel) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		m.mode = "menu"
		m.cursor = 0
		m.message = ""
		return m, nil

	case "tab", "down":
		m.inputField = (m.inputField + 1) % 3

	case "shift+tab", "up":
		m.inputField = (m.inputField - 1 + 3) % 3

	case "enter":
		if m.inputField == 2 && m.inputID != "" && m.inputPwd != "" {
			// Save password
			if err := m.store.Add(m.inputID, m.inputDesc, m.inputPwd); err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
				m.messageType = "error"
			} else {
				if err := m.store.Save(m.masterPwd, nil); err != nil {
					m.message = fmt.Sprintf("Error saving: %v", err)
					m.messageType = "error"
				} else {
					m.message = fmt.Sprintf("Password '%s' added successfully!", m.inputID)
					m.messageType = "success"
					m.passwordAdded = true
					m.inputID = ""
					m.inputDesc = ""
					m.inputPwd = ""
					m.inputField = 0
				}
			}
		}

	case "backspace":
		switch m.inputField {
		case 0:
			if len(m.inputID) > 0 {
				m.inputID = m.inputID[:len(m.inputID)-1]
			}
		case 1:
			if len(m.inputDesc) > 0 {
				m.inputDesc = m.inputDesc[:len(m.inputDesc)-1]
			}
		case 2:
			if len(m.inputPwd) > 0 {
				m.inputPwd = m.inputPwd[:len(m.inputPwd)-1]
			}
		}

	default:
		// Add character to current field
		if len(msg.String()) == 1 {
			switch m.inputField {
			case 0:
				m.inputID += msg.String()
			case 1:
				m.inputDesc += msg.String()
			case 2:
				m.inputPwd += msg.String()
			}
		}
	}

	return m, nil
}

func (m passwordManagerModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc", "q":
		m.mode = "menu"
		m.cursor = 0
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	}

	return m, nil
}

func (m passwordManagerModel) updateRemove(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc", "q":
		m.mode = "menu"
		m.cursor = 0
		m.message = ""
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}

	case "enter", " ":
		if len(m.entries) > 0 && m.cursor < len(m.entries) {
			entry := m.entries[m.cursor]
			if err := m.store.Remove(entry.ID); err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
				m.messageType = "error"
			} else {
				if err := m.store.Save(m.masterPwd, nil); err != nil {
					m.message = fmt.Sprintf("Error saving: %v", err)
					m.messageType = "error"
				} else {
					m.message = fmt.Sprintf("Password '%s' removed", entry.ID)
					m.messageType = "success"
					m.entries = m.store.List()
					if m.cursor >= len(m.entries) {
						m.cursor = len(m.entries) - 1
					}
					if m.cursor < 0 {
						m.cursor = 0
					}
				}
			}
		}
	}

	return m, nil
}

func (m passwordManagerModel) updateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc", "q":
		m.mode = "menu"
		m.cursor = 0
		m.viewingPassword = ""
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.viewingPassword = ""
		}

	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.viewingPassword = ""
		}

	case "enter", " ":
		if len(m.entries) > 0 && m.cursor < len(m.entries) {
			entry := m.entries[m.cursor]
			// Get actual password from store
			pwd, err := m.store.Get(entry.ID)
			if err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
				m.messageType = "error"
			} else {
				m.viewingPassword = pwd
				m.message = ""
			}
		}
	}

	return m, nil
}

func (m passwordManagerModel) updateChangeMaster(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		m.mode = "menu"
		m.cursor = 0
		m.message = ""
		return m, nil

	case "tab", "down":
		m.inputField = (m.inputField + 1) % 3

	case "shift+tab", "up":
		m.inputField = (m.inputField - 1 + 3) % 3

	case "enter":
		if m.inputField == 2 && m.inputOldPwd != "" && m.inputNewPwd != "" {
			// Validate new password
			if len(m.inputNewPwd) < 8 {
				m.message = "New password must be at least 8 characters"
				m.messageType = "error"
				return m, nil
			}

			// Check confirmation
			if m.inputNewPwd != m.inputConfirmPwd {
				m.message = "New passwords do not match"
				m.messageType = "error"
				return m, nil
			}

			// Change master password
			if err := m.store.ChangeMasterPassword(m.inputOldPwd, m.inputNewPwd); err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
				m.messageType = "error"
			} else {
				m.message = "Master password changed successfully!"
				m.messageType = "success"
				m.masterPwd = m.inputNewPwd
				m.inputOldPwd = ""
				m.inputNewPwd = ""
				m.inputConfirmPwd = ""
				m.inputField = 0
			}
		}

	case "backspace":
		switch m.inputField {
		case 0:
			if len(m.inputOldPwd) > 0 {
				m.inputOldPwd = m.inputOldPwd[:len(m.inputOldPwd)-1]
			}
		case 1:
			if len(m.inputNewPwd) > 0 {
				m.inputNewPwd = m.inputNewPwd[:len(m.inputNewPwd)-1]
			}
		case 2:
			if len(m.inputConfirmPwd) > 0 {
				m.inputConfirmPwd = m.inputConfirmPwd[:len(m.inputConfirmPwd)-1]
			}
		}

	default:
		// Add character to current field
		if len(msg.String()) == 1 {
			switch m.inputField {
			case 0:
				m.inputOldPwd += msg.String()
			case 1:
				m.inputNewPwd += msg.String()
			case 2:
				m.inputConfirmPwd += msg.String()
			}
		}
	}

	return m, nil
}

func (m passwordManagerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	switch m.mode {
	case "menu":
		return m.viewMenu()
	case "add":
		return m.viewAdd()
	case "list":
		return m.viewList()
	case "remove":
		return m.viewRemove()
	case "view":
		return m.viewView()
	case "change-master":
		return m.viewChangeMaster()
	}

	return ""
}

func (m passwordManagerModel) viewMenu() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(1, 2)

	menuStyle := lipgloss.NewStyle().
		Padding(1, 2)

	selectedItemStyle := lipgloss.NewStyle().
		Bold(true).
		Reverse(true)

	header := titleStyle.Render("🔐 Password Manager")

	menuItems := []string{
		"Add Password",
		"View Password",
		"List Passwords",
		"Remove Password",
		"Change Master Password",
		"Exit",
	}

	var menuLines []string
	for i, item := range menuItems {
		if i == m.cursor {
			menuLines = append(menuLines, selectedItemStyle.Render("> "+item))
		} else {
			menuLines = append(menuLines, "  "+item)
		}
	}

	menu := menuStyle.Render(strings.Join(menuLines, "\n"))

	footer := footerStyle.Width(m.width).Render("↑↓: Navigate  Enter: Select  q: Quit")

	info := ""
	if m.store.Count() > 0 {
		infoStyle := lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(1, 2)
		info = infoStyle.Render(fmt.Sprintf("Total passwords: %d", m.store.Count()))
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		menu,
		info,
		footer,
	)
}

func (m passwordManagerModel) viewAdd() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(1, 2)

	header := titleStyle.Render("Add New Password")

	formStyle := lipgloss.NewStyle().
		Padding(1, 2)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor)

	inputStyle := lipgloss.NewStyle().
		Foreground(secondaryColor)

	activeInputStyle := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Bold(true).
		Underline(true)

	var formLines []string

	// ID field
	if m.inputField == 0 {
		formLines = append(formLines, labelStyle.Render("ID: ")+activeInputStyle.Render(m.inputID+"█"))
	} else {
		formLines = append(formLines, labelStyle.Render("ID: ")+inputStyle.Render(m.inputID))
	}

	// Description field
	if m.inputField == 1 {
		formLines = append(formLines, labelStyle.Render("Description: ")+activeInputStyle.Render(m.inputDesc+"█"))
	} else {
		formLines = append(formLines, labelStyle.Render("Description: ")+inputStyle.Render(m.inputDesc))
	}

	// Password field
	masked := strings.Repeat("*", len(m.inputPwd))
	if m.inputField == 2 {
		formLines = append(formLines, labelStyle.Render("Password: ")+activeInputStyle.Render(masked+"█"))
	} else {
		formLines = append(formLines, labelStyle.Render("Password: ")+inputStyle.Render(masked))
	}

	form := formStyle.Render(strings.Join(formLines, "\n"))

	// Message
	messageView := ""
	if m.message != "" {
		msgStyle := lipgloss.NewStyle().
			Padding(1, 2).
			Bold(true)

		if m.messageType == "success" {
			msgStyle = msgStyle.Foreground(secondaryColor)
		} else if m.messageType == "error" {
			msgStyle = msgStyle.Foreground(lipgloss.Color("#EF4444"))
		}

		messageView = msgStyle.Render(m.message)
	}

	footer := footerStyle.Width(m.width).Render("Tab: Next Field  Enter: Save  Esc: Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		form,
		messageView,
		footer,
	)
}

func (m passwordManagerModel) viewList() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(1, 2)

	header := titleStyle.Render(fmt.Sprintf("Stored Passwords (%d)", len(m.entries)))

	listStyle := lipgloss.NewStyle().
		Padding(1, 2)

	if len(m.entries) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)
		empty := listStyle.Render(emptyStyle.Render("No passwords stored yet"))
		footer := footerStyle.Width(m.width).Render("Esc: Back")
		return lipgloss.JoinVertical(lipgloss.Left, header, empty, footer)
	}

	var listLines []string
	for i, entry := range m.entries {
		line := fmt.Sprintf("%-20s %s", entry.ID, entry.Description)
		if i == m.cursor {
			listLines = append(listLines, selectedStyle.Render("> "+line))
		} else {
			listLines = append(listLines, "  "+line)
		}
	}

	list := listStyle.Render(strings.Join(listLines, "\n"))
	footer := footerStyle.Width(m.width).Render("↑↓: Navigate  Esc: Back")

	return lipgloss.JoinVertical(lipgloss.Left, header, list, footer)
}

func (m passwordManagerModel) viewRemove() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(1, 2)

	header := titleStyle.Render("Remove Password")

	listStyle := lipgloss.NewStyle().
		Padding(1, 2)

	if len(m.entries) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)
		empty := listStyle.Render(emptyStyle.Render("No passwords to remove"))
		footer := footerStyle.Width(m.width).Render("Esc: Back")

		messageView := ""
		if m.message != "" {
			msgStyle := lipgloss.NewStyle().
				Padding(1, 2).
				Bold(true).
				Foreground(secondaryColor)
			messageView = msgStyle.Render(m.message)
		}

		return lipgloss.JoinVertical(lipgloss.Left, header, empty, messageView, footer)
	}

	var listLines []string
	for i, entry := range m.entries {
		line := fmt.Sprintf("%-20s %s", entry.ID, entry.Description)
		if i == m.cursor {
			listLines = append(listLines, selectedStyle.Render("> "+line))
		} else {
			listLines = append(listLines, "  "+line)
		}
	}

	list := listStyle.Render(strings.Join(listLines, "\n"))

	messageView := ""
	if m.message != "" {
		msgStyle := lipgloss.NewStyle().
			Padding(1, 2).
			Bold(true)

		if m.messageType == "success" {
			msgStyle = msgStyle.Foreground(secondaryColor)
		} else if m.messageType == "error" {
			msgStyle = msgStyle.Foreground(lipgloss.Color("#EF4444"))
		}

		messageView = msgStyle.Render(m.message)
	}

	footer := footerStyle.Width(m.width).Render("↑↓: Navigate  Enter: Remove  Esc: Back")

	return lipgloss.JoinVertical(lipgloss.Left, header, list, messageView, footer)
}

func (m passwordManagerModel) viewView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(1, 2)

	header := titleStyle.Render("View Password")

	listStyle := lipgloss.NewStyle().
		Padding(1, 2)

	if len(m.entries) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)
		empty := listStyle.Render(emptyStyle.Render("No passwords stored yet"))
		footer := footerStyle.Width(m.width).Render("Esc: Back")
		return lipgloss.JoinVertical(lipgloss.Left, header, empty, footer)
	}

	var listLines []string
	for i, entry := range m.entries {
		line := fmt.Sprintf("%-20s %s", entry.ID, entry.Description)
		if i == m.cursor {
			listLines = append(listLines, selectedStyle.Render("> "+line))
		} else {
			listLines = append(listLines, "  "+line)
		}
	}

	list := listStyle.Render(strings.Join(listLines, "\n"))

	// Show password if viewing
	passwordView := ""
	if m.viewingPassword != "" {
		pwdBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2).
			Margin(1, 2).
			Foreground(secondaryColor).
			Bold(true)

		entry := m.entries[m.cursor]
		passwordView = pwdBoxStyle.Render(fmt.Sprintf("Password for '%s':\n\n%s", entry.ID, m.viewingPassword))
	}

	messageView := ""
	if m.message != "" {
		msgStyle := lipgloss.NewStyle().
			Padding(1, 2).
			Bold(true)

		if m.messageType == "error" {
			msgStyle = msgStyle.Foreground(lipgloss.Color("#EF4444"))
		}

		messageView = msgStyle.Render(m.message)
	}

	footer := footerStyle.Width(m.width).Render("↑↓: Navigate  Enter: Show Password  Esc: Back")

	return lipgloss.JoinVertical(lipgloss.Left, header, list, passwordView, messageView, footer)
}

func (m passwordManagerModel) viewChangeMaster() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(1, 2)

	header := titleStyle.Render("Change Master Password")

	formStyle := lipgloss.NewStyle().
		Padding(1, 2)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor)

	inputStyle := lipgloss.NewStyle().
		Foreground(secondaryColor)

	activeInputStyle := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Bold(true).
		Underline(true)

	var formLines []string

	// Old password field
	maskedOld := strings.Repeat("*", len(m.inputOldPwd))
	if m.inputField == 0 {
		formLines = append(formLines, labelStyle.Render("Current Password: ")+activeInputStyle.Render(maskedOld+"█"))
	} else {
		formLines = append(formLines, labelStyle.Render("Current Password: ")+inputStyle.Render(maskedOld))
	}

	// New password field
	maskedNew := strings.Repeat("*", len(m.inputNewPwd))
	if m.inputField == 1 {
		formLines = append(formLines, labelStyle.Render("New Password: ")+activeInputStyle.Render(maskedNew+"█"))
	} else {
		formLines = append(formLines, labelStyle.Render("New Password: ")+inputStyle.Render(maskedNew))
	}

	// Confirm password field
	maskedConfirm := strings.Repeat("*", len(m.inputConfirmPwd))
	if m.inputField == 2 {
		formLines = append(formLines, labelStyle.Render("Confirm Password: ")+activeInputStyle.Render(maskedConfirm+"█"))
	} else {
		formLines = append(formLines, labelStyle.Render("Confirm Password: ")+inputStyle.Render(maskedConfirm))
	}

	form := formStyle.Render(strings.Join(formLines, "\n"))

	// Message
	messageView := ""
	if m.message != "" {
		msgStyle := lipgloss.NewStyle().
			Padding(1, 2).
			Bold(true)

		if m.messageType == "success" {
			msgStyle = msgStyle.Foreground(secondaryColor)
		} else if m.messageType == "error" {
			msgStyle = msgStyle.Foreground(lipgloss.Color("#EF4444"))
		}

		messageView = msgStyle.Render(m.message)
	}

	footer := footerStyle.Width(m.width).Render("Tab: Next Field  Enter: Change  Esc: Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		form,
		messageView,
		footer,
	)
}

// RunPasswordManager starts the password manager TUI
func RunPasswordManager(store *password.PasswordStore, masterPwd string) error {
	m := initialPasswordManagerModel(store, masterPwd)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running password manager: %w", err)
	}

	// Return indication if password was added
	if fm, ok := finalModel.(passwordManagerModel); ok {
		if fm.passwordAdded {
			// Password was added, return nil to indicate success
			return nil
		}
	}

	return nil
}

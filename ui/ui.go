package ui

import (
	"fmt"
	"go-ssh/config"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#10B981")
	accentColor    = lipgloss.Color("#F59E0B")
	dimColor       = lipgloss.Color("#6B7280")
	borderColor    = lipgloss.Color("#4B5563")
)

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderBottom(true).
			BorderForeground(borderColor)

	footerStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(borderColor)

	treeStyle = lipgloss.NewStyle().
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Reverse(true)

	categoryStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	hostStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	descStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)
)

type model struct {
	roots        []*config.TreeNode
	visible      []*config.TreeNode
	cursor       int
	width        int
	height       int
	selectedHost *config.TreeNode
	quitting     bool
}

func initialModel(cfg *config.Config) model {
	roots := config.BuildTree(cfg)
	// Expand first level by default
	for _, root := range roots {
		root.IsExpanded = true
	}
	visible := config.GetVisibleNodes(roots)

	return model{
		roots:   roots,
		visible: visible,
		cursor:  0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}

		case "left", "h":
			if m.cursor < len(m.visible) {
				node := m.visible[m.cursor]
				if node.IsCategory && node.IsExpanded {
					node.IsExpanded = false
					m.visible = config.GetVisibleNodes(m.roots)
				} else if node.Parent != nil {
					// Go to parent
					for i, n := range m.visible {
						if n == node.Parent {
							m.cursor = i
							break
						}
					}
				}
			}

		case "right", "l":
			if m.cursor < len(m.visible) {
				node := m.visible[m.cursor]
				if node.IsCategory && !node.IsExpanded {
					node.IsExpanded = true
					m.visible = config.GetVisibleNodes(m.roots)
				}
			}

		case "enter", " ":
			if m.cursor < len(m.visible) {
				node := m.visible[m.cursor]
				if node.IsCategory {
					node.IsExpanded = !node.IsExpanded
					m.visible = config.GetVisibleNodes(m.roots)
				} else {
					m.selectedHost = node
					return m, tea.Quit
				}
			}

		case "e":
			// Expand all
			expandAll(m.roots, true)
			m.visible = config.GetVisibleNodes(m.roots)

		case "c":
			// Collapse all
			expandAll(m.roots, false)
			m.visible = config.GetVisibleNodes(m.roots)
		}
	}

	return m, nil
}

func expandAll(nodes []*config.TreeNode, expand bool) {
	for _, node := range nodes {
		if node.IsCategory {
			node.IsExpanded = expand
			expandAll(node.Children, expand)
		}
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Header
	headerText := fmt.Sprintf("SSH Host Manager%sHosts: %d",
		strings.Repeat(" ", max(0, m.width-35)),
		countHosts(m.roots))
	header := headerStyle.Width(m.width).Render(headerText)

	// Footer
	footer := footerStyle.Width(m.width).Render(
		"↑↓/jk: Navigate  ←→/hl: Collapse/Expand  Enter: Select  e: Expand All  c: Collapse All  q: Quit",
	)

	// Calculate available height for tree
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	treeHeight := max(5, m.height-headerHeight-footerHeight-1)

	// Tree view
	var treeLines []string
	startIdx := 0
	endIdx := len(m.visible)

	// Scroll if needed
	if len(m.visible) > treeHeight {
		if m.cursor >= treeHeight {
			startIdx = m.cursor - treeHeight + 1
		}
		if startIdx+treeHeight < len(m.visible) {
			endIdx = startIdx + treeHeight
		} else {
			endIdx = len(m.visible)
		}
	}

	for i := startIdx; i < endIdx && i < len(m.visible); i++ {
		node := m.visible[i]
		line := m.renderNode(node, i == m.cursor)

		// Add scroll indicator on the right if needed
		if len(m.visible) > treeHeight {
			relativePos := i - startIdx
			scrollIndicator := m.getScrollIndicator(relativePos, startIdx, endIdx, treeHeight)
			// Pad line to full width minus scroll bar space (account for treeStyle padding and scrollbar)
			paddedLine := lipgloss.NewStyle().Width(m.width - 6).Render(line)
			line = lipgloss.JoinHorizontal(lipgloss.Left, paddedLine, scrollIndicator)
		}

		treeLines = append(treeLines, line)
	}

	// Pad tree to fill space
	for len(treeLines) < treeHeight {
		treeLines = append(treeLines, "")
	}

	tree := treeStyle.Width(m.width).Height(treeHeight).Render(strings.Join(treeLines, "\n"))

	// Combine all
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tree,
		footer,
	)
}

func (m model) renderNode(node *config.TreeNode, selected bool) string {
	// Build indent
	indent := strings.Repeat("  ", node.Level)

	// Build prefix and name with style
	var line string
	if node.IsCategory {
		if node.IsExpanded {
			line = fmt.Sprintf("%s[-] %s", indent, categoryStyle.Render(node.Name))
		} else {
			line = fmt.Sprintf("%s[+] %s", indent, categoryStyle.Render(node.Name))
		}
	} else {
		// Include prefix in styled name so selection highlights both
		line = fmt.Sprintf("%s%s", indent, hostStyle.Render(" ● "+node.Name))
	}

	if selected {
		return selectedStyle.Render("> " + line)
	}

	return "  " + line
}

func (m model) getScrollIndicator(relativePos, startIdx, endIdx, treeHeight int) string {
	totalVisible := len(m.visible)

	// Calculate scroll thumb position and size
	// Thumb size should represent the proportion of visible items
	thumbSize := max(1, (treeHeight*treeHeight+totalVisible-1)/totalVisible)

	// Calculate thumb position based on scroll percentage
	scrollPercentage := float64(startIdx) / float64(max(1, totalVisible-treeHeight))
	thumbStart := int(scrollPercentage * float64(treeHeight-thumbSize))
	thumbEnd := thumbStart + thumbSize

	// Ensure thumb doesn't go out of bounds
	if thumbEnd > treeHeight {
		thumbEnd = treeHeight
		thumbStart = treeHeight - thumbSize
	}

	// Determine scroll indicator character for this line
	var indicator string

	// Check if this line is within the thumb range
	if relativePos >= thumbStart && relativePos < thumbEnd {
		indicator = "█"
	} else if relativePos == 0 && startIdx > 0 {
		// First line with content above
		indicator = "▲"
	} else if relativePos == treeHeight-1 && endIdx < totalVisible {
		// Last line with content below
		indicator = "▼"
	} else {
		// Track line
		indicator = "│"
	}

	return lipgloss.NewStyle().
		Foreground(dimColor).
		Width(2).
		Align(lipgloss.Right).
		Render(indicator)
}

func countHosts(nodes []*config.TreeNode) int {
	count := 0
	for _, node := range nodes {
		count += countHostsInNode(node)
	}
	return count
}

func countHostsInNode(node *config.TreeNode) int {
	if !node.IsCategory {
		return 1
	}
	count := 0
	for _, child := range node.Children {
		count += countHostsInNode(child)
	}
	return count
}

func countCategoriesInNode(node *config.TreeNode) int {
	if !node.IsCategory {
		return 0
	}
	count := 0
	for _, child := range node.Children {
		if child.IsCategory {
			count++
			count += countCategoriesInNode(child)
		}
	}
	return count
}

// Run starts the TUI and returns the selected host command
func Run(cfg *config.Config) (*config.Host, error) {
	m := initialModel(cfg)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running program: %w", err)
	}

	if fm, ok := finalModel.(model); ok {
		if fm.selectedHost != nil {
			return fm.selectedHost.ToHost(), nil
		}
	}

	return nil, nil
}

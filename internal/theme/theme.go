package theme

import (
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	Title     lipgloss.Style
	Brand     lipgloss.Style
	ActiveTab lipgloss.Style
	Tab       lipgloss.Style
	TableHead lipgloss.Style
	Divider   lipgloss.Style
	Selected  lipgloss.Style
	Muted     lipgloss.Style
	Error     lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Footer    lipgloss.Style
	Panel     lipgloss.Style
	Highlight lipgloss.Style
}

func New(noColor bool) Styles {
	if noColor {
		return Styles{}
	}
	return Styles{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "25", Dark: "81"}),
		Brand:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "15"}).Background(lipgloss.AdaptiveColor{Light: "25", Dark: "24"}).Padding(0, 1),
		ActiveTab: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "15"}).Background(lipgloss.AdaptiveColor{Light: "25", Dark: "24"}).Padding(0, 1),
		Tab:       lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "252"}).Padding(0, 1),
		TableHead: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "25", Dark: "81"}),
		Divider:   lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "239"}),
		Selected:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "15"}).Background(lipgloss.AdaptiveColor{Light: "25", Dark: "24"}),
		Muted:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "246"}),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"}),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "114"}),
		Warning:   lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "221"}),
		Footer:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "238", Dark: "252"}),
		Panel:     lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "250", Dark: "239"}),
		Highlight: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "25", Dark: "81"}),
	}
}

func NoColor(flag bool) bool {
	return flag || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
}

func Input(noColor bool) textinput.Model {
	input := textinput.New()
	styles := New(noColor)
	input.PromptStyle = styles.Highlight
	input.TextStyle = lipgloss.Style{}
	input.PlaceholderStyle = styles.Muted
	input.CompletionStyle = styles.Muted
	input.Cursor.Style = lipgloss.Style{}
	input.Cursor.TextStyle = lipgloss.Style{}
	if noColor {
		input.SetCursorMode(textinput.CursorHide)
	}
	return input
}

func SafeText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u202e' || r == '\u202d' || r >= '\u2066' && r <= '\u2069' {
			return -1
		}
		return r
	}, value)
}

package ui

import "github.com/charmbracelet/lipgloss"

var (
	paneStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	titleStyle = lipgloss.NewStyle().Bold(true)
	execStyle  = lipgloss.NewStyle().Faint(true)
	footerHint = lipgloss.NewStyle().Faint(true)
)

// styleDashboard は純コンテンツ（list/menu 等）を受け取り、枠付き 2 ペインに整形する。
// 純コンテンツ層には触れない（テストは純コンテンツ側で行う）。
func styleDashboard(title, list, menu, execLine, footer, status string) string {
	left := paneStyle.Render("Sessions\n" + list)
	right := paneStyle.Render("Actions\n" + menu)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	out := titleStyle.Render(title) + "\n" + body + "\n" +
		execStyle.Render("実行: "+execLine) + "\n" +
		footerHint.Render(footer)
	if status != "" {
		out += "\n" + status
	}
	return out
}

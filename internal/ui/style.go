package ui

import "github.com/charmbracelet/lipgloss"

var (
	paneStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	activePaneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("14"))
	titleStyle      = lipgloss.NewStyle().Bold(true)
	execStyle       = lipgloss.NewStyle().Faint(true)
	runnableStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // 実行可: シアン
	faintStyle      = lipgloss.NewStyle().Faint(true)                      // 実行不可・ヒント: 減光（グレーアウト）
)

// styleDashboard は純コンテンツ（list/menu 等）を受け取り、枠付き 2 ペインに整形する。
// focus==0 で左（Sessions）、focus==1 で右（Actions）の枠を強調する。
// 純コンテンツ層には触れない（テストは純コンテンツ側で行う）。
func styleDashboard(title, list, menu, execLine, footer, status string, focus int) string {
	leftStyle, rightStyle := paneStyle, paneStyle
	if focus == 0 {
		leftStyle = activePaneStyle
	} else {
		rightStyle = activePaneStyle
	}
	left := leftStyle.Render("Sessions\n" + list)
	right := rightStyle.Render("Actions\n" + menu)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	// footer は RenderFooter 側で項目ごとに装飾済み（可=色 / 不可=減光）。ここでは包まない。
	out := titleStyle.Render(title) + "\n" + body + "\n" +
		execStyle.Render("実行: "+execLine) + "\n" +
		footer
	if status != "" {
		out += "\n" + status
	}
	return out
}

package tui

import (
	"strings"

	"treehole/internal/config"

	"charm.land/lipgloss/v2"
)

type ToolsSection int

const (
	ToolsSectionConfig ToolsSection = iota
	ToolsSectionLogs
	ToolsSectionInteractive
	ToolsSectionSystem
	ToolsSectionHelp
)

type ToolsDialogModel struct {
	section       ToolsSection
	Config        ConfigDialogModel
	Logs          LogsDialogModel
	Notifications NotificationDialogModel
}

func NewToolsDialog(cfg *config.Config) ToolsDialogModel {
	return ToolsDialogModel{
		section:       ToolsSectionConfig,
		Config:        NewConfigDialog(cfg),
		Logs:          NewLogsDialog(),
		Notifications: NewNotificationDialog(),
	}
}

func (m ToolsDialogModel) initialized() bool {
	return m.Config.initialized() && m.Logs.initialized() && m.Notifications.initialized()
}

func (m ToolsDialogModel) Section() ToolsSection {
	return m.section
}

func (m *ToolsDialogModel) Switch(section ToolsSection) {
	m.section = section
}

func (m *ToolsDialogModel) View(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	bodyHeight := maxInt(3, height-2)
	switch m.section {
	case ToolsSectionLogs:
		b.WriteString(m.Logs.View(width, bodyHeight))
	case ToolsSectionInteractive, ToolsSectionSystem:
		b.WriteString(m.Notifications.View(width, bodyHeight))
	case ToolsSectionHelp:
		b.WriteString(renderToolsHelp(width, bodyHeight))
	default:
		b.WriteString(m.Config.View(width, bodyHeight))
	}
	rendered := lipgloss.NewStyle().
		Background(colorBg).
		Render(b.String())
	return preserveBackgroundAfterReset(rendered, colorBg)
}

func renderToolsBodyWithFooter(body, footer string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 2 {
		height = 2
	}
	fill := dialogBackgroundFillStyle()
	bodyHeight := height - 1
	body = fillRenderedBackground(body, width, fill)
	body = lipgloss.Place(
		width,
		bodyHeight,
		lipgloss.Left,
		lipgloss.Top,
		body,
		lipgloss.WithWhitespaceStyle(fill),
	)
	footer = clipToVisibleWidth(footer, width)
	footer = vDialogHelpStyle.
		Padding(0).
		Width(width).
		Render(footer)
	footer = fillRenderedBackground(footer, width, fill)
	return preserveBackgroundAfterReset(lipgloss.JoinVertical(lipgloss.Left, body, footer), colorBg)
}

func dialogBackgroundFillStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(colorBg)
}

func (m ToolsDialogModel) renderTabs() string {
	fill := dialogBackgroundFillStyle()
	tabs := []struct {
		label   string
		section ToolsSection
	}{
		{"配置 (1)", ToolsSectionConfig},
		{"日志 (2)", ToolsSectionLogs},
		{"互动 (3)", ToolsSectionInteractive},
		{"系统 (4)", ToolsSectionSystem},
		{"帮助 (?)", ToolsSectionHelp},
	}
	parts := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		style := vStatLabelStyle
		if m.section == tab.section {
			style = vStatValueStyle
		}
		parts = append(parts, style.Background(colorBg).Render(tab.label))
	}
	return strings.Join(parts, fill.Render("  "))
}

func renderToolsHelp(width, height int) string {
	var lines []string
	add := func(line string) {
		lines = append(lines, clipToVisibleWidth(line, width))
	}
	addHeading := func(line string) {
		lines = append(lines, vStatValueStyle.Render(clipToVisibleWidth(line, width)))
	}

	addHeading("项目用法")
	add("")
	add("启动后先进入 Dashboard，可查看未读通知、热榜和常用入口。")
	add("按 e 进入浏览；按 c 编辑配置；按 n 打开互动通知；按 ? 打开本帮助。")
	add("进入帖子列表后，可搜索、打开详情、查看图片、发布帖子、点赞或关注。")
	add("")
	addHeading("全局快捷键")
	add("Tab: 切换主页面")
	add("?: 项目帮助")
	add("h: 当前页面快捷键")
	add("c: 配置")
	add("l: 日志")
	add("b: 通知")
	add("Ctrl+Q: 退出")
	add("")
	addHeading("帖子快捷键")
	add("↑↓: 选择/滚动")
	add("PgUp/PgDn: 快速翻页")
	add("/: 搜索")
	add("Enter: 打开详情")
	add("o: 打开图片")
	add("r: 刷新")
	add("n: 发布帖子")
	add("p: 点赞")
	add("f: 关注")
	add("t: 标签筛选")

	body := strings.Join(lines, "\n")
	return renderToolsBodyWithFooter(body, "?: 帮助 | h: 当前页快捷键 | Esc: 关闭", width, height)
}

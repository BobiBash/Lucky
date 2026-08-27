package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	// "strings"
	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	textAreaHorizontalOverhead = 6
	viewportVerticalOverhead   = 5
	InputHorizontalOverhead    = 4
	InputVerticalOverhead      = 6
	TimerHorizontalOverhead    = 2
)

type tickMsg time.Time

type responseMsg struct {
	Content string
}

type CmdMenu struct {
	Content string
}

type Message struct {
	Role    string
	Content string
}

type commandItem struct {
	title string
	desc  string
}

func (i commandItem) Title() string       { return i.title }
func (i commandItem) Description() string { return i.desc }
func (i commandItem) FilterValue() string { return i.title }

type model struct {
	textarea    textarea.Model
	viewport    viewport.Model
	messages    []Message
	waiting     bool
	err         error
	elapsed     int
	cursor      int
	commandList []commandItem
	showMenu    bool
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// var cmd []tea.Cmd

	userStyle := lipgloss.NewStyle().
		Background(lipgloss.Black).
		MarginBottom(1).
		Width(m.textarea.Width()).
		Padding(1, 0, 1, 1)
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.textarea.SetWidth(msg.Width - textAreaHorizontalOverhead)
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height - m.textarea.Height() - viewportVerticalOverhead - TimerHorizontalOverhead)

	case responseMsg:

		elapsedDisplayStyle := lipgloss.NewStyle().Faint(true)

		timeElapsed := elapsedDisplayStyle.Render(fmt.Sprintf("Time elapsed: %s", strconv.Itoa(m.elapsed)))
		m.messages = append(m.messages, Message{Role: "assistant", Content: msg.Content + "\n" + timeElapsed})
		var content strings.Builder

		for _, msg := range m.messages {
			switch msg.Role {
			case "user":
				fmt.Fprintf(&content, "%s\n", userStyle.Render(msg.Content))
			case "assistant":
				fmt.Fprintf(&content, "%s\n", msg.Content)
			}
		}

		m.viewport.GotoBottom()
		m.viewport.SetContent(content.String())
		m.viewport.Update(msg)
		m.waiting = false
		return m, m.textarea.Focus()

	case tickMsg:
		m.elapsed += 1
		if !m.waiting {
			return m, nil
		}
		return m, tick()

	case tea.KeyPressMsg:
		if m.showMenu {
			switch msg.String() {

			}
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			m.elapsed = 0
			userContent := m.textarea.Value()
			m.messages = append(m.messages, Message{Role: "user", Content: userContent})

			var content strings.Builder
			for _, msg := range m.messages {
				switch msg.Role {
				case "user":
					fmt.Fprintf(&content, "%s\n", userStyle.Render(msg.Content))
				case "assistant":
					fmt.Fprintf(&content, "%s\n", msg.Content)
				}
			}

			m.viewport.SetContent(content.String())
			m.textarea.Reset()
			m.textarea.Blur()
			m.waiting = true
			return m, tea.Batch(CallAPI(&m, userContent), tick())

		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			if strings.HasPrefix(m.textarea.Value(), "/") {
				m.showMenu = true
			} else {
				m.showMenu = false
			}
			return m, cmd
		}

	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case cursor.BlinkMsg:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd

	}

	return m, nil
}

func (m model) View() tea.View {
	var timerArea string
	viewportView := m.viewport.View()

	textareaStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder())

	textareaView := textareaStyle.Render(m.textarea.View())

	centeredTextArea := lipgloss.PlaceHorizontal(m.viewport.Width(), lipgloss.Center, textareaView)

	if m.waiting {
		timerStyle := lipgloss.NewStyle().
			MarginRight(3)

		timer := fmt.Sprintf("%s seconds", strconv.Itoa(m.elapsed))
		timerView := timerStyle.Render(timer)

		timerArea = lipgloss.JoinVertical(lipgloss.Right, timerView, centeredTextArea)
	} else {
		timerStyle := lipgloss.NewStyle().Height(1)
		timerView := timerStyle.Render()
		timerArea = lipgloss.JoinVertical(lipgloss.Right, timerView, centeredTextArea)
	}

	var cmdMenuArea string
	if m.showMenu {
		cmdMenu := m.RenderCommands()
		cmdMenuArea = lipgloss.JoinVertical(lipgloss.Left, cmdMenu, centeredTextArea)
	} else {
		cmdMenu := lipgloss.NewStyle().Height(5)
		cmdMenuView := cmdMenu.Render()
		cmdMenuArea = lipgloss.JoinVertical(lipgloss.Left, cmdMenuView, centeredTextArea)
	}

	v := tea.NewView(viewportView + "\n" + cmdMenuArea + timerArea + centeredTextArea)
	c := m.textarea.Cursor()

	if c != nil {
		c.Y += lipgloss.Height(viewportView) + InputVerticalOverhead
		c.X += InputHorizontalOverhead
	}
	v.Cursor = c
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	return v
}

func main() {

	p := tea.NewProgram(InitialModel())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func InitialModel() model {

	taPromptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("5"))

	ta := textarea.New()

	taStyle := ta.Styles()
	taStyle.Focused.Prompt = taPromptStyle
	ta.SetStyles(taStyle)
	ta.SetVirtualCursor(false)
	ta.Focus()
	ta.ShowLineNumbers = false

	ta.Prompt = ""
	ta.CharLimit = 300
	// ta.SetStyles(taStyle)

	ta.SetHeight(2)

	vp := viewport.New()
	vpStyle := lipgloss.NewStyle().
		MarginLeft(3).
		MarginRight(3).
		MarginTop(2)

	vp.Style = vpStyle

	cmds := []commandItem{
		{title: "/help", desc: "Display all commands."},
		{title: "/test", desc: "Test command."},
	}

	return model{
		textarea:    ta,
		viewport:    vp,
		commandList: cmds,
	}
}

func CallAPI(m *model, userContent string) tea.Cmd {
	return func() tea.Msg {
		godotenv.Load()
		apiKey := os.Getenv("apiKey")
		client := openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL("https://api.xiaomimimo.com/v1"))
		personality, err := os.ReadFile("Personality")

		if err != nil {
			log.Fatal(err)
		}

		chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(string(personality)),
				openai.UserMessage(userContent),
			},
			Model: "xiaomi/mimo-v2.5",
		})

		if err != nil {
			panic(err)
		}

		return responseMsg{Content: chatCompletion.Choices[0].Message.Content}
	}

}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// TODO: add keybinds to move through the cmds, make scrollable via slicing(start:end - where end := start + maxVisible)
func (m model) RenderCommands() string {
	lines := make([]string, len(m.commandList))

	cmdsStyle := lipgloss.NewStyle().Height(5)

	for index, cmd := range m.commandList {
		prefix := " "

		if m.cursor == index {
			prefix = ">"
		}

		lines[index] = fmt.Sprintf("%s %s %s", prefix, cmd.title, cmd.desc)
	}

	cmdList := strings.Join(lines, "\n")
	cmdView := cmdsStyle.Render(cmdList)

	return cmdView
}

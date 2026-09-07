package main

import (
	"context"
	"errors"
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
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	textAreaHorizontalOverhead = 6
	viewportVerticalOverhead   = 3
	InputHorizontalOverhead    = 4
	InputVerticalOverhead      = 2
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

type Cursor struct {
	cursor int
	start  int
}

type model struct {
	baseViewPortHeight int
	textarea           textarea.Model
	viewport           viewport.Model
	messages           []Message
	waiting            bool
	err                error
	elapsed            int
	cursor             Cursor
	commandList        []commandItem
	showMenu           bool
	cfg                Config
	client             openai.Client
	personality        string
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// var cmd []tea.Cmd
	maxVisible := 8

	// User msg style
	userStyle := lipgloss.NewStyle().
		Background(lipgloss.Black).
		MarginBottom(1).
		Width(m.textarea.Width()).
		Padding(1, 0, 1, 1)

	switch msg := msg.(type) {

	// Initial window size and resizing
	case tea.WindowSizeMsg:
		m.textarea.SetWidth(msg.Width - textAreaHorizontalOverhead)
		m.viewport.SetWidth(msg.Width)
		m.baseViewPortHeight = msg.Height - m.textarea.Height() - viewportVerticalOverhead
		m.viewport.SetHeight(m.baseViewPortHeight)

	// Response message by the AI
	case responseMsg:

		elapsedDisplayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

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

	// Timer
	case tickMsg:
		m.elapsed += 1
		if !m.waiting {
			return m, nil
		}
		return m, tick()

	// Keypresses
	case tea.KeyPressMsg:
		// Cmd Menu keypresses
		if m.showMenu {
			switch msg.String() {
			case "up":

				if m.cursor.cursor <= 0 {
					m.cursor.cursor = len(m.commandList)
					m.cursor.start = len(m.commandList) - maxVisible
				}

				if m.cursor.cursor > 0 {
					m.cursor.cursor--
					if m.cursor.cursor < m.cursor.start+4 {
						if m.cursor.start > 0 {
							m.cursor.start--
						}
					}
				}
				return m, nil
			case "down":

				if m.cursor.cursor >= len(m.commandList)-1 {
					m.cursor.cursor = -1
					m.cursor.start = 0
				}

				if m.cursor.cursor < len(m.commandList)-1 {
					m.cursor.cursor++
					if m.cursor.cursor > 3 && m.cursor.start < len(m.commandList)-maxVisible {
						m.cursor.start++
					}
				}
				return m, nil
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
			return m, tea.Batch(CallAPI(m, userContent), tick())

		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			if strings.HasPrefix(m.textarea.Value(), "/") {
				m.showMenu = true
				m.viewport.SetHeight(m.baseViewPortHeight - 7)
			} else {
				m.showMenu = false
				m.viewport.SetHeight(m.baseViewPortHeight)
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

	textareaStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder())

	textareaView := textareaStyle.Render(m.textarea.View())

	centeredTextArea := lipgloss.PlaceHorizontal(m.viewport.Width(), lipgloss.Center, textareaView)

	// Timer View
	var timerView string

	if m.waiting {
		timerStyle := lipgloss.NewStyle().
			MarginRight(3)

		timer := fmt.Sprintf("%s seconds", strconv.Itoa(m.elapsed))
		timerView = timerStyle.Render(timer)

	} else {
		timerStyle := lipgloss.NewStyle().Height(1)
		timerView = timerStyle.Render()
	}

	// Commands Menu
	var cmdMenuArea string

	if m.showMenu {
		cmdMenuArea = lipgloss.NewStyle().Render(m.RenderCommands())
	}

	InputSection := lipgloss.JoinVertical(lipgloss.Right, timerView, centeredTextArea)

	viewportView := m.viewport.View()
	v := tea.NewView(viewportView + "\n" + cmdMenuArea + InputSection)
	c := m.textarea.Cursor()

	// cursor positioning
	if c != nil {
		cmdMenuVerticalOverhead := 7
		if m.showMenu {
			c.Y += lipgloss.Height(viewportView) + InputVerticalOverhead + cmdMenuVerticalOverhead
		} else {
			c.Y += lipgloss.Height(viewportView) + InputVerticalOverhead
		}
		c.X += InputHorizontalOverhead - 1
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

	var cfg Config

	path, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		CLI()
	} else {
		cfg, err = loadConfig(path)

		if err != nil {
			Setup()
			cfg, err = loadConfig(path)
			if err != nil {
				log.Fatalf("Error loading config: %v", err)
			}
		}
	}

	cfg = CheckConfig(path)

	client := openai.NewClient(
		option.WithAPIKey(cfg.ApiKey),
		option.WithBaseURL(cfg.BaseUrl))
	personality, err := os.ReadFile("Personality")

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
		{title: "/test", desc: "Test command1."},

		{title: "/test", desc: "Test command2."},

		{title: "/test", desc: "Test command3."},
		{title: "/test", desc: "Test command4."},
		{title: "/test", desc: "Test command5."},
		{title: "/test", desc: "Test command6a."},
		{title: "/test", desc: "Test command7."},
		{title: "/test", desc: "Test command8."},
		{title: "/test", desc: "Test command9."},
		{title: "/test", desc: "Test command10."},
		{title: "/test", desc: "Test command11."},

		{title: "/test", desc: "Test command12."},
		{title: "/test", desc: "Test command13."},
		{title: "/test", desc: "Test command14."},
		{title: "/test", desc: "Test command15."},
		{title: "/test", desc: "Test command16."},
		{title: "/test", desc: "Test command17."},
		{title: "/test", desc: "Test command18."},
		{title: "/test", desc: "Test command19."},
	}

	return model{
		personality: string(personality),
		client:      client,
		cfg:         cfg,
		textarea:    ta,
		viewport:    vp,
		commandList: cmds,
	}
}

func CallAPI(m model, userContent string) tea.Cmd {
	return func() tea.Msg {

		chatCompletion, err := m.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(string(m.personality)),
				openai.UserMessage(userContent),
			},
			Model: m.cfg.Model,
		})

		if err != nil {
			var ApiError *openai.Error

			if errors.As(err, &ApiError) {
				return responseMsg{Content: fmt.Sprintf("Error: %v", ApiError.Message)}
			}
			return responseMsg{Content: fmt.Sprintf("Error: %v", err)}
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

	var lineTitleStyle lipgloss.Style
	var lineDescStyle lipgloss.Style
	cmdsStyle := lipgloss.NewStyle().Height(8).MarginLeft(1)

	for index, cmd := range m.commandList {
		prefix := " "
		lineTitleStyle = lipgloss.NewStyle()
		lineDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

		if m.cursor.cursor == index {
			prefix = ">"
			lineTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1E81E8"))

		}
		title := lineTitleStyle.Render(fmt.Sprintf("%s %s  ", prefix, cmd.title))
		desc := lineDescStyle.Render(fmt.Sprintf("%s", cmd.desc))
		lines[index] = title + desc
	}

	const maxVisible = 8
	// Read the persistent start from model
	// start := m.cursor.start
	// start = max(0, len(m.commandList) - 5)
	end := min(m.cursor.start+maxVisible, len(m.commandList))

	cmdSlice := lines[m.cursor.start:end]
	cmdList := strings.Join(cmdSlice, "\n")
	cmdView := cmdsStyle.Render(cmdList)

	return cmdView
}

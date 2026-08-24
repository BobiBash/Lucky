package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	// "strings"
	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

const (
	textAreaHorizontalOverhead = 8
	viewportVerticalOverhead   = 2
	InputHorizontalOverhead    = 4
	InputVerticalOverhead      = 1
)

type responseMsg struct {
	Content string
}

type Message struct {
	Role    string
	Content string
}

type model struct {
	textarea textarea.Model
	viewport viewport.Model
	messages []Message
	waiting  bool
	err      error
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.textarea.SetWidth(msg.Width - textAreaHorizontalOverhead)
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height - m.textarea.Height() - viewportVerticalOverhead)
	case responseMsg:
		m.messages = append(m.messages, Message{Role: "assistant", Content: msg.Content})
		var content strings.Builder

		for _, msg := range m.messages {
			fmt.Fprintf(&content, "%s\n", msg.Content)
		}

		m.viewport.GotoBottom()
		m.viewport.SetContent(content.String())
		m.viewport.Update(msg)
		return m, m.textarea.Focus()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			userContent := m.textarea.Value()
			m.messages = append(m.messages, Message{Role: "user", Content: userContent})

			var content strings.Builder
			for _, msg := range m.messages {
				fmt.Fprintf(&content, "%s\n", msg.Content)
			}

			m.viewport.SetContent(content.String())
			m.textarea.Reset()
			m.textarea.Blur()
			return m, CallAPI(&m, userContent)

		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
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
	viewportView := m.viewport.View()

	textareaStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderLeft(true)

	textareaView := textareaStyle.Render(m.textarea.View())
	centeredTextArea := lipgloss.PlaceHorizontal(m.viewport.Width(), lipgloss.Center, textareaView)

	v := tea.NewView(viewportView + "\n" + centeredTextArea)
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

	return model{
		textarea: ta,
		viewport: vp,
	}
}

func CallAPI(m *model, userContent string) tea.Cmd {
	return func() tea.Msg {
		godotenv.Load()
		apiKey := os.Getenv("apiKey")
		ctx := context.Background()
		client := openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL("https://api.xiaomimimo.com/v1"))

		resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
			Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(userContent)},
			Model: "xiaomi/mimo-v2.5",
		})

		if err != nil {
			panic(err)
		}

		return responseMsg{Content: resp.OutputText()}
	}

}

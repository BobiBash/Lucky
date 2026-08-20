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
		m.textarea.SetWidth(msg.Width)
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height - m.textarea.Height())
	case responseMsg:
		m.messages = append(m.messages, Message{Role: "assistant", Content: msg.Content})
		var content strings.Builder

		for _, msg := range m.messages {
			fmt.Fprintf(&content, "%s\n", msg.Content)
		}

		m.viewport.SetContent(content.String())
		return m, m.textarea.Focus()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			userContent := m.textarea.Value()
			m.messages = append(m.messages, Message{Role: "user", Content: userContent})

			var firstContent strings.Builder
			for _, msg := range m.messages {
				fmt.Fprintf(&firstContent, "%s\n", msg.Content)
			}

			m.viewport.SetContent(firstContent.String())

			var content strings.Builder
			for _, msg := range m.messages {
				fmt.Fprintf(&content, "%s\n", msg.Content)
			}

			m.viewport.SetContent(content.String())
			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.textarea.Blur()
			return m, CallAPI(&m, userContent)

		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

	case cursor.BlinkMsg:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd

	}

	return m, nil
}

func (m model) View() tea.View {
	viewportView := m.viewport.View()

	v := tea.NewView(viewportView + "\n" + m.textarea.View())
	c := m.textarea.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(viewportView)
	}
	v.Cursor = c
	v.AltScreen = true

	return v
}

func main() {
	p := tea.NewProgram(InitialModel())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func InitialModel() model {
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder())

	ta := textarea.New()
	ta.SetVirtualCursor(false)
	ta.Focus()

	ta.Prompt = "| "
	ta.CharLimit = 300

	ta.SetHeight(2)

	vp := viewport.New()
	vp.Style = style

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

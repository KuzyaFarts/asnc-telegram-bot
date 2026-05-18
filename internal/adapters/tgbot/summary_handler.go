package tgbot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const summaryHeader = "📋 <b>Саммари за сегодня</b>\n\n"

func (tb *Bot) onSummary(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) {
		return
	}

	if tb.openAIKey == "" {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "OPENAI_API_KEY не настроен.", tb.ttl)
		return
	}

	chatMsgs, err := tb.msgStore.GetTodayMessages(ctx, msg.Chat.ID)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Ошибка при получении сообщений.", tb.ttl)
		return
	}
	if len(chatMsgs) == 0 {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Сегодня ещё нет сообщений для саммари.", tb.ttl)
		return
	}

	var conv strings.Builder
	for _, m := range chatMsgs {
		name := m.FirstName
		if m.Username != "" {
			name = "@" + m.Username
		}
		conv.WriteString(name)
		conv.WriteString(": ")
		conv.WriteString(m.Text)
		conv.WriteByte('\n')
	}

	sent, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    msg.Chat.ID,
		Text:      summaryHeader + "⏳ Генерирую...",
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		return
	}

	var (
		mu    sync.Mutex
		accum strings.Builder
	)

	done := make(chan struct{})

	// Throttle Telegram edits to ~1 per 800ms to avoid rate limits
	go func() {
		ticker := time.NewTicker(800 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				text := accum.String()
				mu.Unlock()
				if text == "" {
					continue
				}
				b.EditMessageText(ctx, &bot.EditMessageTextParams{
					ChatID:    msg.Chat.ID,
					MessageID: sent.ID,
					Text:      summaryHeader + text + " ▌",
					ParseMode: models.ParseModeHTML,
				})
			case <-done:
				return
			}
		}
	}()

	streamErr := openAISummarizeStream(ctx, tb.openAIKey, conv.String(), func(chunk string) {
		mu.Lock()
		accum.WriteString(chunk)
		mu.Unlock()
	})
	close(done)

	mu.Lock()
	finalText := accum.String()
	mu.Unlock()

	if streamErr != nil || finalText == "" {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    msg.Chat.ID,
			MessageID: sent.ID,
			Text:      summaryHeader + "Не удалось получить саммари от OpenAI.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    msg.Chat.ID,
		MessageID: sent.ID,
		Text:      summaryHeader + finalText,
		ParseMode: models.ParseModeHTML,
	})
}

type openAIRequest struct {
	Model               string          `json:"model"`
	Stream              bool            `json:"stream"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Messages            []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func openAISummarizeStream(ctx context.Context, apiKey, chat string, onChunk func(string)) error {
	reqBody := openAIRequest{
		Model:               "gpt-5",
		Stream:              true,
		MaxCompletionTokens: 5500,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: `Ты — циничный летописец чата. Не помощник, не суммаризатор — стендапер, который наблюдает за стадом и рассказывает что они опять натворили. Свысока, с подколом, но любя — это твои кореша. Стиль: живой разговорный русский, мат можно (без перегиба), сленг, ирония. Пиши грамотно — должно быть смешно, а не криво.

КРАТКО. Без воды. Максимум 2-3 коротких абзаца, каждый по 2-4 предложения. Длинные полотна никто читать не будет. Лучше плотно и метко, чем долго и пресно.

Правила:
- Открывай дайджест одной хлёсткой фразой ("Ну шо, погнали", "Сегодня в дурдоме всё как обычно", "Ловите сводку" — придумывай своё).
- 2-3 абзаца, каждый — главная ветка разговора. Группируй по темам, не по людям. Мелкие переписки игнорируй.
- Между абзацами один динамичный переход — "Параллельно", "А следом", "Под конец" — короткий, не разворачивай.
- Цитируй только реально смешные моменты — одну-две цитаты на весь дайджест, не больше.
- НЕ философствуй, не оправдывай, не подводи мораль. Только факты в стёбной обёртке.
- Заверши одной короткой едкой фразой.
- Имена и ники пиши БЕЗ символа @ — "Иван", "Шлерп". Чтобы никого не пинговало.

Пример нужной краткости (имена вымышленные):
"Ну шо, погнали. День открыл Вася — три часа объяснял почему 'задача на 5 минут' не делается за 5 минут. Параллельно Лёха завёл шарманку про новую видяху, Петя добил: 'ты её год выбираешь, а играешь в косынку'. Под конец Маша скинула рецепт борща в чат про код. Классика."`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Вот переписка за сегодня:\n\n%s\nСделай краткое саммари.", chat),
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return fmt.Errorf("openai error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}
	return scanner.Err()
}

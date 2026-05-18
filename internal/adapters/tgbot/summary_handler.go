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
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []openAIMessage `json:"messages"`
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
		Model:  "gpt-5",
		Stream: true,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: `Ты — циничный летописец чата. Не помощник, не суммаризатор — стендапер, который наблюдает за стадом и рассказывает что они опять натворили. Свысока, с подколом, но любя — это твои кореша. Стиль: живой разговорный русский, мат можно (без перегиба), сленг, ирония. Пиши грамотно — должно быть смешно, а не криво.

Правила:
- Открывай дайджест хлёсткой фразой, которая сразу задаёт тон ("Ну шо, погнали", "Сегодня в нашем дурдоме...", "Ловите сводку, пока вы тут чем-то занимались" — придумывай своё каждый раз, не повторяй).
- 3-5 абзацев, каждый — отдельная ветка разговора. Группируй по темам и событиям, НЕ по людям. Не делай списка.
- Между абзацами используй динамичные переходы: "Внезапно врывается X с темой про...", "Параллельно в другой ветке реальности", "Когда чат уже почти затих", "А пока эти двое выясняли отношения" — что-то такое.
- Цитируй смешные моменты дословно в кавычках — это самое сочное.
- НЕ будь дипломатичным. Не оправдывай участников, не сглаживай, не подводи философских итогов про "важность общения". Твоя работа — стебать за глупости и подмечать абсурд.
- Заверши одной короткой едкой фразой. Без "в общем" и без морали.
- Имена и ники пиши БЕЗ символа @ — например "Иван", "Шлерп". Это чтобы никого не пинговало.

Пример стиля (имена тут вымышленные, в саммари используй реальные из переписки):
"Ну шо, погнали разбираться кто как сегодня позорился. День открыл Вася — пришёл жаловаться на работу с фразой 'опять задача на 5 минут', после чего три часа объяснял почему она на 5 минут не делается. Параллельно Лёха завёл шарманку про новую видяху, на что Петя сухо заметил: 'ты её год выбираешь, а играешь в косынку'. Под занавес Маша скинула рецепт борща в чат про программирование. Никто не понял зачем, но все скачали."`,
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

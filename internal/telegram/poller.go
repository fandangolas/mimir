package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/fandangolas/mimir/internal/observability"
)

// IncomingMessage is a simplified representation of a Telegram message
// passed to the handler.
type IncomingMessage struct {
	ChatID    int64
	MessageID int
	Text      string
}

// Handler is the function called for each incoming text message.
type Handler func(ctx context.Context, msg IncomingMessage)

// Poll starts long polling for updates and calls handler for each text message.
// Only messages from allowedUserIDs are dispatched; all others are silently dropped.
// It blocks until ctx is cancelled and all in-flight handlers have finished.
func (c *Client) Poll(ctx context.Context, allowedUserIDs map[int64]struct{}, handler Handler) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := c.bot.GetUpdatesChan(u)

	slog.Info("telegram polling started", "bot", c.bot.Self.UserName)

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			c.bot.StopReceivingUpdates()
			slog.Info("telegram polling stopped, draining in-flight messages...")
			wg.Wait()
			slog.Info("all in-flight messages drained")
			return

		case update, ok := <-updates:
			if !ok {
				wg.Wait()
				return
			}
			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			userID := update.Message.From.ID
			if _, allowed := allowedUserIDs[userID]; !allowed {
				slog.Warn("rejected message from unauthorized user", "user_id", userID)
				observability.UnauthorizedMessages.Inc()
				continue
			}

			msg := IncomingMessage{
				ChatID:    update.Message.Chat.ID,
				MessageID: update.Message.MessageID,
				Text:      update.Message.Text,
			}

			// Attach a correlation ID per message for log tracing.
			correlationID := fmt.Sprintf("%d-%d", msg.ChatID, msg.MessageID)
			msgCtx := observability.WithCorrelationID(ctx, correlationID)

			// Handle each message in its own goroutine so slow LLM responses
			// don't block the polling loop.
			wg.Add(1)
			go func() {
				defer wg.Done()
				handler(msgCtx, msg)
			}()
		}
	}
}

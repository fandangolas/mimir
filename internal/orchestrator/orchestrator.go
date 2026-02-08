package orchestrator

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/fandangolas/personal-assistant/internal/llm"
	"github.com/fandangolas/personal-assistant/internal/observability"
	"github.com/fandangolas/personal-assistant/internal/store"
	"github.com/fandangolas/personal-assistant/internal/telegram"
)

// Orchestrator coordinates the full message lifecycle:
// Telegram message → conversation history → LLM → reply → Telegram.
type Orchestrator struct {
	tg                 *telegram.Client
	chat               llm.ChatProvider
	db                 *store.DB
	conversationWindow int
}

// New creates a new Orchestrator.
func New(tg *telegram.Client, chat llm.ChatProvider, db *store.DB, conversationWindow int) *Orchestrator {
	return &Orchestrator{
		tg:                 tg,
		chat:               chat,
		db:                 db,
		conversationWindow: conversationWindow,
	}
}

// Handle processes a single incoming Telegram message.
func (o *Orchestrator) Handle(ctx context.Context, msg telegram.IncomingMessage) {
	tracer := otel.Tracer("orchestrator")
	ctx, span := tracer.Start(ctx, "handle_message")
	defer span.End()

	span.SetAttributes(attribute.Int64("chat_id", msg.ChatID))

	log := observability.FromContext(ctx).With("chat_id", msg.ChatID)

	observability.MessagesReceived.Inc()

	sessionID, err := o.db.GetOrCreateSession(ctx, msg.ChatID)
	if err != nil {
		log.Error("failed to get/create session", "error", err)
		observability.MessagesProcessed.WithLabelValues("error").Inc()
		_ = o.tg.Send(msg.ChatID, "Sorry, I'm having trouble right now. Please try again.")
		return
	}

	if err := o.db.SaveMessage(ctx, sessionID, "user", msg.Text); err != nil {
		log.Error("failed to save user message", "error", err)
	}

	history, err := o.db.GetRecentMessages(ctx, sessionID, o.conversationWindow)
	if err != nil {
		log.Error("failed to retrieve conversation history", "error", err)
		observability.MessagesProcessed.WithLabelValues("error").Inc()
		_ = o.tg.Send(msg.ChatID, "Sorry, I'm having trouble right now. Please try again.")
		return
	}

	llmMessages := buildMessages(history)

	log.Info("calling LLM", "messages", len(llmMessages))

	start := time.Now()
	reply, err := o.chat.Chat(ctx, llmMessages)
	observability.LLMLatency.Observe(time.Since(start).Seconds())

	if err != nil {
		log.Error("LLM call failed", "error", err)
		observability.MessagesProcessed.WithLabelValues("error").Inc()
		_ = o.tg.Send(msg.ChatID, fmt.Sprintf("Sorry, I couldn't generate a response: %v", err))
		return
	}

	if err := o.db.SaveMessage(ctx, sessionID, "assistant", reply); err != nil {
		log.Error("failed to save assistant message", "error", err)
	}

	if err := o.tg.Send(msg.ChatID, reply); err != nil {
		log.Error("failed to send telegram reply", "error", err)
		observability.TelegramSendErrors.Inc()
	}

	observability.MessagesProcessed.WithLabelValues("success").Inc()
	log.Info("message handled", "llm_duration_ms", time.Since(start).Milliseconds())
}

// buildMessages converts stored messages into the LLM message format.
func buildMessages(history []store.Message) []llm.Message {
	msgs := make([]llm.Message, 0, len(history)+1)

	msgs = append(msgs, llm.Message{
		Role:    llm.RoleSystem,
		Content: "You are a helpful personal assistant. Be concise and direct.",
	})

	for _, m := range history {
		msgs = append(msgs, llm.Message{
			Role:    llm.Role(m.Role),
			Content: m.Content,
		})
	}

	return msgs
}

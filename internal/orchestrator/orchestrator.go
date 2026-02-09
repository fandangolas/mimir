package orchestrator

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/fandangolas/mimir/internal/llm"
	"github.com/fandangolas/mimir/internal/observability"
	"github.com/fandangolas/mimir/internal/rag"
	"github.com/fandangolas/mimir/internal/store"
	"github.com/fandangolas/mimir/internal/telegram"
)

// Orchestrator coordinates the full message lifecycle:
// Telegram message → conversation history → LLM → reply → Telegram.
type Orchestrator struct {
	tg                 *telegram.Client
	chat               llm.ChatProvider
	db                 *store.DB
	conversationWindow int
	ragPipeline        *rag.Pipeline // Optional RAG pipeline (nil if disabled)
}

// New creates a new Orchestrator.
func New(tg *telegram.Client, chat llm.ChatProvider, db *store.DB, conversationWindow int) *Orchestrator {
	return &Orchestrator{
		tg:                 tg,
		chat:               chat,
		db:                 db,
		conversationWindow: conversationWindow,
		ragPipeline:        nil,
	}
}

// WithRAG adds RAG pipeline support to the orchestrator.
func (o *Orchestrator) WithRAG(ragPipeline *rag.Pipeline) *Orchestrator {
	o.ragPipeline = ragPipeline
	return o
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

	// Use RAG pipeline if available, otherwise use traditional approach
	var llmMessages []llm.Message
	var reply string

	if o.ragPipeline != nil && o.ragPipeline.IsEnabled() {
		log.Info("using RAG pipeline for context")

		// RAG pipeline handles message saving with embeddings
		_, err := o.ragPipeline.StoreMessageWithEmbedding(ctx, sessionID, "user", msg.Text)
		if err != nil {
			log.Error("failed to save user message with RAG", "error", err)
		}

		// Build context with RAG
		llmMessages, err = o.ragPipeline.BuildContextWithRAG(ctx, sessionID, msg.Text)
		if err != nil {
			log.Error("failed to build RAG context", "error", err)
			observability.MessagesProcessed.WithLabelValues("error").Inc()
			_ = o.tg.Send(msg.ChatID, "Sorry, I'm having trouble right now. Please try again.")
			return
		}
	} else {
		log.Info("using traditional context (RAG disabled)")

		// Traditional: save message without embeddings
		if err := o.db.SaveMessage(ctx, sessionID, "user", msg.Text); err != nil {
			log.Error("failed to save user message", "error", err)
		}

		// Traditional: build context from recent messages
		history, err := o.db.GetRecentMessages(ctx, sessionID, o.conversationWindow)
		if err != nil {
			log.Error("failed to retrieve conversation history", "error", err)
			observability.MessagesProcessed.WithLabelValues("error").Inc()
			_ = o.tg.Send(msg.ChatID, "Sorry, I'm having trouble right now. Please try again.")
			return
		}

		llmMessages = buildMessages(history)
	}

	log.Info("calling LLM", "messages", len(llmMessages))

	start := time.Now()
	reply, err = o.chat.Chat(ctx, llmMessages)
	observability.LLMLatency.Observe(time.Since(start).Seconds())

	if err != nil {
		log.Error("LLM call failed", "error", err)
		observability.MessagesProcessed.WithLabelValues("error").Inc()
		_ = o.tg.Send(msg.ChatID, fmt.Sprintf("Sorry, I couldn't generate a response: %v", err))
		return
	}

	// Save assistant reply (with embedding if RAG is enabled)
	if o.ragPipeline != nil && o.ragPipeline.IsEnabled() {
		_, err := o.ragPipeline.StoreMessageWithEmbedding(ctx, sessionID, "assistant", reply)
		if err != nil {
			log.Error("failed to save assistant message with RAG", "error", err)
		}
	} else {
		if err := o.db.SaveMessage(ctx, sessionID, "assistant", reply); err != nil {
			log.Error("failed to save assistant message", "error", err)
		}
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

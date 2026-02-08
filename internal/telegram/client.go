package telegram

import (
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Client wraps the Telegram bot API with a circuit breaker on sends.
type Client struct {
	bot     *tgbotapi.BotAPI
	breaker *gobreaker.CircuitBreaker[tgbotapi.Message]
}

// New creates a new Telegram client and validates the token.
func New(token string) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}

	cb := gobreaker.NewCircuitBreaker[tgbotapi.Message](gobreaker.Settings{
		Name:        "telegram-send",
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})

	return &Client{bot: bot, breaker: cb}, nil
}

// Send sends a plain text message to the given chat ID.
func (c *Client) Send(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown

	_, err := c.breaker.Execute(func() (tgbotapi.Message, error) {
		return c.bot.Send(msg)
	})
	if err != nil {
		return fmt.Errorf("sending telegram message to %d: %w", chatID, err)
	}
	return nil
}

// Username returns the bot's username (useful for logging).
func (c *Client) Username() string {
	return c.bot.Self.UserName
}

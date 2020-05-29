package telegram

import (
	"net/http"
	"time"

	tb "github.com/go-telegram-bot-api/telegram-bot-api"
)

func New(token string, enableDebug bool) (*tb.BotAPI, tb.UpdateConfig, error) {
	bot, err := tb.NewBotAPIWithClient(token, &http.Client{Timeout: 5 * time.Second})
	bot.Debug = enableDebug
	u := tb.NewUpdate(0)
	u.Timeout = 60
	return bot, u, err
}

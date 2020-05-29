package telegram

import (
	tb "github.com/go-telegram-bot-api/telegram-bot-api"
)

func New(token string, enableDebug bool) (*tb.BotAPI, tb.UpdateConfig, error) {
	bot, err := tb.NewBotAPI(token)
	bot.Debug = enableDebug
	u := tb.NewUpdate(0)
	u.Timeout = 5
	return bot, u, err
}

package telegram

import (
	tb "github.com/go-telegram-bot-api/telegram-bot-api"
)

func New(token string, enableDebug bool) (*tb.BotAPI, error) {
	bot, err := tb.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = enableDebug
	return bot, err
}

func UpdateConfig(offset, timeout int) tb.UpdateConfig {
	u := tb.NewUpdate(offset)
	u.Timeout = timeout
	return u
}

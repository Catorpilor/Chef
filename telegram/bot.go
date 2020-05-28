package telegram

import (
	"net/http"
	"time"

	"github.com/prometheus/common/log"
	tb "gopkg.in/tucnak/telebot.v2"
)

func New(token string) (*tb.Bot, error) {
	bot, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		Client: &http.Client{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}
	bot.Handle("/add", func(m *tb.Message) {
		log.Infof("get message with content: %#v", m)
		bot.Send(m.Sender, "Got your message")
	})
	return bot, nil
}

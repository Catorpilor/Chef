package guard

import (
	"fmt"
	"time"

	"github.com/catorpilor/idenaMgrBot/idena"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
)

const (
	defaultInterval = 15 * time.Minute
)

type Guardian struct {
	bot      *tgbotapi.BotAPI
	watched  map[string]int64
	ticker   *time.Ticker
	idenaCtl *idena.Client
}

func New(bot *tgbotapi.BotAPI, ctl *idena.Client) *Guardian {
	return &Guardian{
		bot:      bot,
		watched:  make(map[string]int64),
		ticker:   time.NewTicker(defaultInterval),
		idenaCtl: ctl,
	}
}

func (guard *Guardian) isWatched(addr string) bool {
	if _, exists := guard.watched[addr]; exists {
		return true
	}
	return false
}

func (guard *Guardian) Add(addr string, chatID int64) {
	if guard.isWatched(addr) {
		return
	}
	guard.watched[addr] = chatID
}

func (guard *Guardian) Start() {
	go func() {
		for {
			select {
			case <-guard.ticker.C:
				for addr, chatID := range guard.watched {
					go func(addr string, chatID int64) {
						os := guard.idenaCtl.CheckOnlineIdentity(addr)
						log.Infof("calling addr:%s got %v", addr, os)
						if os != nil && !os.Online {
							//
							msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("addr:%s is offline and lastActivity is %s",
								addr, os.LastActivity))
							// send message
							_, _ = guard.bot.Send(msg)
						}
					}(addr, chatID)
				}
			}
		}

	}()
}

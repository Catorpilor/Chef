package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"

	localhttp "github.com/catorpilor/idenaMgrBot/http"
	"github.com/catorpilor/idenaMgrBot/idena"
	"github.com/catorpilor/idenaMgrBot/pkg/guard"
	"github.com/catorpilor/idenaMgrBot/redis"
	"github.com/catorpilor/idenaMgrBot/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
)

func main() {
	bot, uc, err := telegram.New(`1057482486:AAH2r2OqGsu9yrkKOEcQ2j1RXl6AWeceIas`, false)
	if err != nil {
		log.Fatal(err)
	}
	lc := idena.NewCtl(nil)
	rc := redis.NewPool("localhost:6379", 3)
	watcher := guard.New(bot, lc)
	go watcher.Start()
	go func() {
		updates, err := bot.GetUpdatesChan(uc)
		if err != nil {
			log.Infof("getUpdatesChan got with config:%v got err:%s", uc, err.Error())
		}
		for update := range updates {
			if update.Message == nil {
				continue
			}
			log.Infof("got message: [%s] %v", update.Message.From.UserName, update.Message)
			if update.Message.IsCommand() {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				switch update.Message.Command() {
				case "help":
					msg.Text = "some help messages"
				case "add":
					log.Infof("message context: %s", update.Message.CommandArguments())
					rawargs := update.Message.CommandArguments()
					args := strings.Fields(rawargs)
					if len(args) > 1 || len(args) < 1 {
						msg.Text = "invalid argumenets, wanted one"
						break
					}
					if !lc.ValidateAddress(args[0]) {
						msg.Text = "invalid idena address"
					} else {
						watcher.Add(args[0], update.Message.Chat.ID)
						msg.Text = "successly added address to the watch list"
					}
				}
				msg.ReplyToMessageID = update.Message.MessageID
				_, err := bot.Send(msg)
				if err != nil {
					log.Infof("send message:%v got err:%s", msg, err.Error())
				}
			}
		}
	}()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	httpsrv := localhttp.NewServer(":8090")
	go func() {
		<-interrupt
		log.Info("graceful server")
		if err = httpsrv.Shutdown(context.Background()); err != nil {
			log.Infof("could not shutdown: %v", err)
			return
		}
	}()
	log.Info("starting http server on :8090")
	err = httpsrv.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatal(err)
	}

}

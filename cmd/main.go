package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	localhttp "github.com/catorpilor/idenaMgrBot/http"
	"github.com/catorpilor/idenaMgrBot/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
)

func main() {
	bot, uc, err := telegram.New(`1059453367:AAGdpYFGxw5mAYOTjRIr9_i-ZI_vY4BOcOU`, true)
	if err != nil {
		log.Fatal(err)
	}
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
					msg.Text = "Success"
				}
				bot.Send(msg)
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

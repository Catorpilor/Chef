package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	localhttp "github.com/catorpilor/idenaMgrBot/http"
	"github.com/catorpilor/idenaMgrBot/telegram"
	log "github.com/sirupsen/logrus"
)

func main() {
	bot, err := telegram.New(`1059453367:AAGdpYFGxw5mAYOTjRIr9_i-ZI_vY4BOcOU`)
	if err != nil {
		log.Fatal(err)
	}
	go bot.Start()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	httpsrv := localhttp.NewServer(":8080")
	go func() {
		<-interrupt
		log.Info("graceful server")
		if err = httpsrv.Shutdown(context.Background()); err != nil {
			log.Infof("could not shutdown: %v", err)
			return
		}
	}()
	log.Info("starting http server on :8080")
	err = httpsrv.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatal(err)
	}

}

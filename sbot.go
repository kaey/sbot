// Copyright 2016 Konstantin Kulikov. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command sbot is a stupid SB bot.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	_ "github.com/denisenkom/go-mssqldb"
	"github.com/jmoiron/sqlx"
	"github.com/tucnak/telebot"
)

var (
	pidPath  = flag.String("pid", "", "Path to pid file. if empty, pid is not written.")
	logPath  = flag.String("log", "", "Path to log file. If empty, log goes to stderr.")
	confPath = flag.String("config", "sbot.conf", "Path to config file")
)

// Config is a program configuration.
type Config struct {
	DB     string
	Token  string
	ChatID int64
}

// TTSMessage represent single TTS message.
type TTSMessage struct {
	ID           int    `db:"MSGID"`
	Text         string `db:"MSGText"`
	Author       string `db:"MSGAuthor"`
	SectionID    int    `db:"MSGSectionID"`
	SubsectionID int    `db:"MSGSubSectionID"`
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	if err := writePID(*pidPath); err != nil {
		log.Fatalln(err)
	}

	if err := writeLog(*logPath); err != nil {
		log.Fatalln(err)
	}

	var conf Config
	if _, err := toml.DecodeFile(*confPath, &conf); err != nil {
		log.Fatalln(err)
	}

	ttsdb, err := sqlx.Open("mssql", conf.DB)
	if err != nil {
		log.Fatalln(err)
	}
	defer ttsdb.Close()

	c, err := buildSBChain(ttsdb)
	if err != nil {
		log.Fatalln(err)
	}

	var lastProcessedID int
	if err := ttsdb.Get(&lastProcessedID, "select top 1 MSGID from MSGS order by MSGID DESC"); err != nil {
		log.Fatalln(err)
	}

	bot, err := telebot.NewBot(conf.Token)
	if err != nil {
		log.Fatalln(err)
	}

	messages := make(chan telebot.Message)
	ticker := time.NewTicker(5 * time.Minute)
	chat := telebot.Chat{
		ID:   conf.ChatID,
		Type: "group",
	}
	pollQ := "select MSGID,MSGText,MSGAuthor,MSGSectionID,MSGSubSectionID from MSGS where CreatedBy in (10,109) and MSGSectionID in (3,6,10) and MSGID > $1 order by MSGID ASC"

	bot.Listen(messages, 1*time.Second)
	log.Println("Serving")

	for {
		select {
		case msg := <-messages:
			if msg.Chat.ID != chat.ID {
				continue
			}

			if len(strings.Split(msg.Text, " ")) != 1 {
				continue
			}

			text := c.GenerateWithKeyword(msg.Text, 100)
			if text == "" {
				bot.SendMessage(msg.Chat, "Что?", nil)
				continue
			}
			bot.SendMessage(msg.Chat, text, nil)
		case <-ticker.C:
			if chat.ID == 0 {
				continue
			}
			var messages []TTSMessage
			if err := ttsdb.Select(&messages, pollQ, lastProcessedID); err != nil {
				log.Println(err)
				continue
			}
			if len(messages) == 0 {
				continue
			}
			lastProcessedID = messages[len(messages)-1].ID
			log.Printf("got %v messages, last ID: %v", len(messages), lastProcessedID)
			for _, msg := range filterMessages(messages) {
				bot.SendMessage(
					chat,
					fmt.Sprintf("%s:\n%s", msg.Author, msg.Text),
					nil,
				)
			}
		}
	}
}

func buildSBChain(ttsdb *sqlx.DB) (*Chain, error) {
	q := "select MSGID,MSGText,MSGAuthor,MSGSectionID,MSGSubSectionID from MSGS where CreatedBy in (10) and MSGSectionID in (3,6,10) and MSGDate > $1 order by MSGID ASC"
	var messages []TTSMessage
	if err := ttsdb.Select(&messages, q, time.Now().Add(-1*2*365*24*time.Hour)); err != nil {
		return nil, err
	}

	c := NewChain(3)
	for _, msg := range filterMessages(messages) {
		c.Build(strings.NewReader(msg.Text))
	}

	return c, nil
}

func filterMessages(messages []TTSMessage) []TTSMessage {
	var filteredMessages []TTSMessage
	for _, msg := range messages {
		if strings.HasPrefix(msg.Text, "Назначен ответственный") {
			continue
		}
		if strings.HasPrefix(msg.Text, "Назначен исполнитель") {
			continue
		}
		if strings.HasPrefix(msg.Text, "Ответственный ") {
			continue
		}
		if strings.HasPrefix(msg.Text, "Закрытие заявки") {
			continue
		}
		if strings.HasPrefix(msg.Text, "Инцидент") {
			continue
		}
		if strings.HasPrefix(msg.Text, "Заявка") {
			continue
		}
		if strings.HasPrefix(msg.Text, "Клиенту отправлено сообщение:") {
			continue
		}
		if len(strings.Split(msg.Text, " ")) < 3 {
			continue
		}
		filteredMessages = append(filteredMessages, msg)
	}

	return filteredMessages
}

func writePID(path string) error {
	if path == "" {
		return nil
	}
	pid := fmt.Sprintf("%v", os.Getpid())
	return ioutil.WriteFile(path, []byte(pid), 0644)
}

func writeLog(path string) error {
	if path == "" {
		return nil
	}
	os.Stdin.Close()
	os.Stdout.Close()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	if err := syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd())); err != nil {
		return err
	}
	return nil
}

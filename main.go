package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mxk/go-imap/imap"
)

type Config struct {
	Server      string
	User        string
	Password    string
	Destination string
}

type Goatee struct {
	config Config
	client *imap.Client
}

func (g *Goatee) Connect() {
	var err error
	log.Print("Connecting to server..\n")
	g.client, err = imap.DialTLS(g.config.Server, &tls.Config{})

	if err != nil {
		log.Fatalf("Connection to server failed: %s", err)
	}

	if g.client.State() == imap.Login {
		log.Print("Logging in..\n")
		g.client.Login(g.config.User, g.config.Password)
	}

	log.Print("Opening INBOX..\n")
	g.client.Select("INBOX", true)
}

func (g *Goatee) ExtractAttachment(msg *mail.Message, params map[string]string) {
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Error parsing part: %s", err)
		}

		ct := p.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/pdf") {
			path := filepath.Join(".", g.config.Destination,
				p.FileName())
			dst, err := os.Create(path)
			if err != nil {
				log.Fatalf("Failed to create file: %s", err)
			}
			r := base64.NewDecoder(base64.StdEncoding, p)
			_, err = io.Copy(dst, r)
			if err != nil {
				log.Fatalf("Failed to store attachment: %s", err)
			}
		}
	}
}

func (g *Goatee) FetchMails() {
	set, _ := imap.NewSeqSet("1:*")
	log.Print("Fetching mails..\n")
	cmd, err := g.client.Fetch(set, "BODY[]")

	if err != nil {
		log.Fatalf("Fetch failed: %s", err)
	}

	for cmd.InProgress() {
		g.client.Recv(10 * time.Second)

		for _, rsp := range cmd.Data {
			body := imap.AsBytes(rsp.MessageInfo().Attrs["BODY[]"])

			if msg, _ := mail.ReadMessage(bytes.NewReader(body)); msg != nil {
				fmt.Println("|--", msg.Header.Get("Subject"))
				mediaType, params, _ := mime.ParseMediaType(
					msg.Header.Get("Content-Type"))
				if strings.HasPrefix(mediaType, "multipart/") {
					g.ExtractAttachment(msg, params)
				}
			}
		}
		cmd.Data = nil
	}

	if rsp, err := cmd.Result(imap.OK); err != nil {
		if err == imap.ErrAborted {
			log.Fatal("Fetch command aborted")
		} else {
			log.Fatalf("Fetch error:", rsp.Info)
		}
	}
}

func (g *Goatee) ReadConfig(filename string) {
	if _, err := toml.DecodeFile(filename, &g.config); err != nil {
		log.Fatalf("Error opening config file: %s", err)
	}
}

func main() {
	g := Goatee{}
	g.ReadConfig("goatee.cfg")
	g.Connect()
	defer g.client.Logout(30 * time.Second)
	g.FetchMails()
}

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

func main() {
	var config Config
	if _, err := toml.DecodeFile("goatee.cfg", &config); err != nil {
		log.Fatalf("Error opening config file: %s", err)
	}

	log.Print("Connecting to server..\n")
	c, err := imap.DialTLS(config.Server, &tls.Config{})

	if err != nil {
		log.Fatalf("Connection to server failed: %s", err)
	}

	defer c.Logout(30 * time.Second)

	if c.State() == imap.Login {
		log.Print("Logging in..\n")
		c.Login(config.User, config.Password)
	}

	log.Print("Opening INBOX..\n")
	c.Select("INBOX", true)

	set, _ := imap.NewSeqSet("1:*")
	log.Print("Fetching mails..\n")
	cmd, err := c.Fetch(set, "BODY[]")

	if err != nil {
		log.Fatalf("Fetch failed: %s", err)
	}

	for cmd.InProgress() {
		c.Recv(10 * time.Second)

		for _, rsp := range cmd.Data {
			header := imap.AsBytes(rsp.MessageInfo().Attrs["BODY[]"])

			if msg, _ := mail.ReadMessage(bytes.NewReader(header)); msg != nil {
				fmt.Println("|--", msg.Header.Get("Subject"))
				mediaType, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
				if strings.HasPrefix(mediaType, "multipart/") {
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
							path := filepath.Join(".", config.Destination,
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
			}
		}
		cmd.Data = nil
		c.Data = nil
	}

	if rsp, err := cmd.Result(imap.OK); err != nil {
		if err == imap.ErrAborted {
			log.Fatal("Fetch command aborted")
		} else {
			log.Fatalf("Fetch error:", rsp.Info)
		}
	}

}

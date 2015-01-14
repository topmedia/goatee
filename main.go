package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"flag"
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
	"github.com/alexcesaro/quotedprintable"
	"github.com/mxk/go-imap/imap"
)

var wd, _ = os.Getwd()
var conf = flag.String("conf", wd+"/goatee.cfg", "Path to config file.")
var logfile = flag.String("log", wd+"/goatee.log", "Path to log file.")
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
	g.client.Select("INBOX", false)
}

func (g *Goatee) DecodeSubject(msg *mail.Message) string {
	s, _, err := quotedprintable.DecodeHeader(msg.Header.Get("Subject"))

	if err != nil {
		return msg.Header.Get("Subject")
	} else {
		return s
	}
}

func (g *Goatee) ExtractAttachment(r io.Reader, params map[string]string) {
	mr := multipart.NewReader(r, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Error parsing part: %s", err)
		}

		ct, params, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))

		if strings.HasPrefix(ct, "multipart/mixed") {
			log.Printf("Extracting attachments from %s", ct)
			g.ExtractAttachment(p, params)
		} else if strings.HasPrefix(ct, "application/pdf") {
			path := filepath.Join(wd, g.config.Destination,
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
	log.Print("Fetching unread UIDs..\n")
	cmd, err := g.client.UIDSearch("1:* NOT SEEN")
	cmd.Result(imap.OK)

	if err != nil {
		log.Fatalf("UIDSearch failed: %s", err)
	}

	uids := cmd.Data[0].SearchResults()
	if len(uids) == 0 {
		log.Fatal("No unread messages found.")
	}

	log.Print("Fetching mail bodies..\n")
	set, _ := imap.NewSeqSet("")
	set.AddNum(uids...)
	cmd, err = g.client.Fetch(set, "UID", "FLAGS", "BODY[]")

	if err != nil {
		log.Fatalf("Fetch failed: %s", err)
	}

	for cmd.InProgress() {
		g.client.Recv(10 * time.Second)

		for _, rsp := range cmd.Data {
			body := imap.AsBytes(rsp.MessageInfo().Attrs["BODY[]"])

			if msg, _ := mail.ReadMessage(bytes.NewReader(body)); msg != nil {
				log.Printf("|-- %v", g.DecodeSubject(msg))
				mediaType, params, _ := mime.ParseMediaType(
					msg.Header.Get("Content-Type"))
				if strings.HasPrefix(mediaType, "multipart/") {
					log.Printf("Extracting attachments from %s", mediaType)
					g.ExtractAttachment(msg.Body, params)
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

	log.Print("Marking messages seen..\n")
	cmd, err = g.client.UIDStore(set, "+FLAGS.SILENT",
		imap.NewFlagSet(`\Seen`))

	if rsp, err := cmd.Result(imap.OK); err != nil {
		log.Fatalf("UIDStore error:", rsp.Info)
	}

	cmd.Data = nil
}

func (g *Goatee) OpenLog(path string) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Error opening logfile: %v", err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
}

func (g *Goatee) ReadConfig(path string) {
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("File doesn't exist: %v", err)
	}

	if _, err := toml.DecodeFile(path, &g.config); err != nil {
		log.Fatalf("Error opening config file: %s", err)
	}
}

func main() {
	flag.Parse()
	g := Goatee{}
	g.Connect()
	defer g.client.Logout(30 * time.Second)
	g.FetchMails()
	g.ReadConfig(*conf)
	g.OpenLog(*logfile)
}

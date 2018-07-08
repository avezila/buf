package parser

import (
	"io/ioutil"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap/client"

	imap "github.com/emersion/go-imap"
	"github.com/pkg/errors"
)

func _WaitForMail(restr string, box string) (text string, err error) {
	re, err := regexp.Compile(restr)
	if err != nil {
		return "", errors.WithStack(err)
	}

	c, err := client.DialTLS(ENV_IMAP_HREF, nil)
	if err != nil {
		return "", errors.Wrap(err, "Failed DialTLS")
	}
	// log.Println("Connected")

	// Don't forget to logout
	defer c.Logout()

	// Login
	if err := c.Login(ENV_IMAP_USER, ENV_IMAP_PASSWORD); err != nil {
		return "", errors.Wrap(err, "Failed Login")
	}
	// log.Println("Logged in")

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	// log.Println("Mailboxes:")
	// for m := range mailboxes {
	// 	log.Println("* " + m.Name)
	// }

	if err := <-done; err != nil {
		return "", errors.Wrap(err, "Failed <-done")
	}

	// Select INBOX
	mbox, err := c.Select(box, true)
	if err != nil {
		return "", errors.Wrap(err, "Failed Select Inbox")
	}
	// log.Println("Flags for INBOX:", mbox.Flags)

	// Get the last 4 messages
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > 3 {
		// We're using unsigned integers here, only substract if the result is > 0
		from = mbox.Messages - 3
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done = make(chan error, 1)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	// log.Println("Last 4 messages:")
	for msg := range messages {

		r := msg.GetBody(section)
		if r == nil {
			// log.Println("Server didn't returned message body")
			continue
		}

		m, err := mail.ReadMessage(r)
		if err != nil {
			// log.Println("Failed mail.ReadMessage", err)
			continue
		}
		body, err := ioutil.ReadAll(quotedprintable.NewReader(m.Body))
		if err != nil {
			continue
		}
		sbody := string(body)
		// log.Println(sbody)
		if re.MatchString(sbody) {
			return sbody, nil
		}
	}

	if err := <-done; err != nil {
		return "", errors.Wrap(err, "Failed <-done")
	}

	// log.Println("loop!")

	return "", nil
}
func WaitForMail(restr string) (text string, err error) {
	for i := 0; i < 20; i++ {
		folders := strings.Split(ENV_IMAP_FOLDERS, ",")
		for _, folder := range folders {
			mail, err := _WaitForMail(restr, folder)
			if err != nil {
				return "", err
			}
			if mail != "" {
				return mail, nil
			}
		}
		time.Sleep(time.Second * 3)
	}
	return "", errors.New("Wait for mail timedout")
}

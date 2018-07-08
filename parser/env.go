package parser

import (
	"log"
	"net/url"
	"os"

	"github.com/goware/urlx"
	"github.com/pkg/errors"
)

var ENV_IMAP_HREF = os.Getenv("IMAP_HREF")
var ENV_CAPTCHA_DECODE_KEY = os.Getenv("CAPTCHA_DECODE_KEY")
var ENV_CAPTCHA_DECODE_URL = os.Getenv("CAPTCHA_DECODE_URL")

var ENV_LOCAL_IP = os.Getenv("LOCAL_IP")
var ENV_DOMAIN = os.Getenv("DOMAIN")
var ENV_PROTOCOL = os.Getenv("PROTOCOL")
var ENV_HTTP_PORT = os.Getenv("HTTP_PORT")
var ENV_SERVICE_PORT = os.Getenv("SERVICE_PORT")

var ENV_DB_HOST = os.Getenv("DB_HOST")
var ENV_DB_NAME = os.Getenv("DB_NAME")
var ENV_DB_USER = os.Getenv("DB_USER")
var ENV_DB_PASS = os.Getenv("DB_PASS")
var ENV_DB_MAX_CONNECTIONS = os.Getenv("DB_MAX_CONNECTIONS")

var ENV_IMAP_USER = os.Getenv("IMAP_USER")
var ENV_IMAP_PASSWORD = os.Getenv("IMAP_PASSWORD")
var ENV_IMAP_FOLDERS = os.Getenv("IMAP_FOLDERS")

var ENV_TOR_CLIENT_PARSE_PORT = os.Getenv("TOR_CLIENT_PARSE_PORT")
var ENV_TOR_CLIENT_AUDIO_PORT = os.Getenv("TOR_CLIENT_AUDIO_PORT")

var ENV_MAIN_PROXY = os.Getenv("MAIN_PROXY")
var ENV_RUTRACKER_HOST = os.Getenv("RUTRACKER_HOST")

var ProxyURL *url.URL

func init() {
	if ENV_MAIN_PROXY != "" {
		proxyUrl, err := urlx.Parse(ENV_MAIN_PROXY)
		if err != nil {
			log.Printf("Failed parse proxy '%s': %+v\n", ENV_MAIN_PROXY, errors.WithStack(err))
		}
		ProxyURL = proxyUrl
	}
}

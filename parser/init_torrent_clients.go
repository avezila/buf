package parser

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/anacrolix/dht"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
	"github.com/pkg/errors"
)

const TORRENTS_PATH_PARSE = "/torrents_data/parse"
const GITHUB_ANONCERS = `https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_best.txt`

var AnonceTrackers = [][]string{}

var torParseClient *torrent.Client

func initTorentClients() error {
	res, err := http.Get(GITHUB_ANONCERS)
	if err != nil {
		return errors.Wrap(err, "Failed get github anoncers")
	}
	defer res.Body.Close()
	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	for _, f := range strings.Fields(string(buf)) {
		AnonceTrackers = append(AnonceTrackers, []string{f})
	}
	// log.Println(AnonceTrackers)
	if err := os.MkdirAll(TORRENTS_PATH_PARSE, os.ModePerm); err != nil {
		log.Printf("%+v\n", errors.WithStack(err))
	}
	// log.Println(s.trackers)
	torParseClient, err = torrent.NewClient(&torrent.Config{
		// DefaultStorage: storage.NewFile(TORRENTS_PATH), // storage.NewBoltDB(TORRENTS_PATH),
		DefaultStorage: storage.NewFileByInfoHash(TORRENTS_PATH_PARSE),
		DHTConfig: dht.ServerConfig{
			StartingNodes: dht.GlobalBootstrapAddrs,
		},
		ListenAddr: ENV_LOCAL_IP + ":" + ENV_TOR_CLIENT_PARSE_PORT,
		// DisableTrackers:         true,
		Debug:                   false,
		Seed:                    false,
		NoUpload:                true,
		DisableAggressiveUpload: true,
		DisableIPv6:             true,
		EncryptionPolicy: torrent.EncryptionPolicy{
			PreferNoEncryption: true,
		},
		EstablishedConnsPerTorrent: 200,
	})
	if err != nil {
		return errors.Wrap(err, "Failed torrent.NewClient for parsers")
	}
	return nil
}

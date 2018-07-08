package parser

import (
	"log"
	"regexp"
	"time"

	"github.com/jackc/pgx"
	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
)

var ruTrackerRegexp = regexp.MustCompile("<b>Вход</b></a>")
var nnmClubRegexp = regexp.MustCompile("Вход</a>")
var golangRegexp = regexp.MustCompile("Found</a>")

func checkWorkingProxy(url string, proxyIP string, find *regexp.Regexp) bool {
	_, body, errs := gorequest.New().Proxy(proxyIP).End()

	if len(errs) != 0 {
		return false
	}

	if find != nil && find.FindStringIndex(body) == nil {
		return false
	}
	return true
}

//JobCheckProxy checks proxy for some problems with different usages
//proxy ip specified by req
//It returns error if you shouldnt commit changes in tx
func JobCheckProxy(req *JobRequest, tx *pgx.Tx) error {
	if req.IP == nil {
		log.Printf("nil ip in job check proxy %+v\n", req)
		return nil
	}
	proxy := &Proxy{}
	proxy.IP = req.IP
	ip := *req.IP

	for trackerName, tracker := range Trackers {
		switch trackerName {
		case RuTracker:
			works := checkWorkingProxy(tracker.Domain+"forum/login.php", "http://"+ip, ruTrackerRegexp)
			if works {
				proxy.DomainRuTracker = &works
			}
		case NNMClub:
			works := checkWorkingProxy(tracker.Domain+"forum/login.php", "http://"+ip, nnmClubRegexp)
			if works {
				proxy.DomainNNMClub = &works
			}
		}
	}

	delayStart := time.Now()
	golangWorks := checkWorkingProxy("http://golang.org/", "http://"+ip, golangRegexp)
	if golangWorks {
		unGolangWorks := !golangWorks
		proxy.Broken = &unGolangWorks
	}

	delay := float32(time.Since(delayStart).Seconds())
	proxy.Delay = &delay

	httpsWorks := checkWorkingProxy("https://duckduckgo.com/", "https://"+ip, nil)
	if httpsWorks {
		proxy.Broken = &httpsWorks
	}
	query, queryArgs := proxy.Fields().NotNil().Upsert("gongo.proxy", "ip")
	if query == "" {
		return nil
	}
	commandTag, err := tx.Exec(query, queryArgs...)
	if err != nil {
		return errors.Wrap(err, "JobCheckProxy cant exec update")
	}

	if commandTag.RowsAffected() != 1 {
		return errors.New("No row found to update")
	}

	return nil
}

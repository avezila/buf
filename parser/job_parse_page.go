package parser

import (
	"log"
	"strings"
	"time"

	"github.com/AlekSi/pointer"

	"github.com/PuerkitoBio/purell"
	"github.com/goware/urlx"
	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

func JobParsePage(pctx *Context, tx *pgx.Tx, req *JobRequest) error {
	if req.HREF == nil {
		return errors.Wrap(ErrBadJob, "nil href")
	}
	u, err := urlx.Parse(*req.HREF)
	if err != nil {
		return errors.Wrap(ErrSkipJob, "bad href "+*req.HREF)
	}
	timestart := time.Now()
	defer func() {
		log.Printf("PARSED %.3f %s\n", time.Now().Sub(timestart).Seconds(), u.String())
	}()

	if strings.Contains(u.Hostname(), "rutracker") {
		return ParseRutracker(pctx, tx, req, u)
	}
	return errors.Wrap(ErrSkipJob, "no info yet how to parse this url")
}

func RequestParsePage(pctx *Context, url string, priority float32) error {
	tx, err := pctx.Dbx.Begin()
	if err != nil {
		return errors.Wrap(err, "Failed begin pgx.Tx")
	}
	defer tx.Rollback()
	if err := JobMaybeParsePage(tx, url, priority, true); err != nil {
		return err
	}

	return errors.Wrap(tx.Commit(), "Failed commit transation for add job for parse href")
}

func JobMaybeParsePage(tx *pgx.Tx, href string, priority float32, force bool) error {
	href, err := URLNormalize(href)
	// log.Println(href)
	if err != nil {
		return errors.Wrap(err, "failed JobMaybeParsePage:URLNormalize")
	}
	uhref, err := urlx.Parse(href)
	if err != nil {
		return errors.Wrap(err, "failed JobMaybeParsePage:url.Parse")
	}
	if strings.Contains(uhref.Host, "rutracker") {
		uhref.Host = ENV_RUTRACKER_HOST
	}
	var found string
	var trackerName *string = nil
	var jobName *string = nil
	var trackerTorrentId *string = nil
	if strings.Contains(uhref.Hostname(), "rutracker") {
		trackerName = pointer.ToString("rutracker")
	}
	jobName = pointer.ToString(uhref.Path)
	if uhref.Path == "/forum/viewtopic.php" || uhref.Path == "/forum/dl.php" {
		trackerTorrentId = pointer.ToString(uhref.Query().Get("t"))
	}
	if !force {
		if err := tx.QueryRow(`
			SELECT id FROM gongo.parse_queue
			WHERE job_type = 'PARSE_PAGE' and href = $1 and add_time > now() - interval '7 day'
		`, href).Scan(&found); err != pgx.ErrNoRows {
			if err != nil {
				return errors.Wrap(err, "failed check href exists in last 7 days")
			}
			return nil
		}
	}
	_, err = tx.Exec(`
		INSERT INTO gongo.parse_queue
			(job_type, href, priority, tracker_name, job_name, tracker_torrent_id)
		VALUES
			('PARSE_PAGE',$1, $2,$3,$4,$5)
	`, href, priority, trackerName, jobName, trackerTorrentId)
	if err != nil {
		return errors.Wrap(err, "failed add href to parse queue")
	}
	return nil
}

func URLNormalize(url string) (string, error) {
	return purell.NormalizeURLString(
		url,
		purell.FlagLowercaseScheme|
			purell.FlagLowercaseHost|
			purell.FlagUppercaseEscapes|
			purell.FlagDecodeUnnecessaryEscapes|
			purell.FlagEncodeNecessaryEscapes|
			purell.FlagRemoveDefaultPort|
			purell.FlagRemoveEmptyQuerySeparator|
			purell.FlagRemoveDotSegments|
			purell.FlagRemoveFragment|
			purell.FlagRemoveDuplicateSlashes|
			purell.FlagSortQuery|
			purell.FlagDecodeDWORDHost|
			purell.FlagDecodeOctalHost|
			purell.FlagDecodeHexHost|
			purell.FlagRemoveUnnecessaryHostDots|
			purell.FlagRemoveEmptyPortSeparator|
			purell.FlagForceHTTP,
	)
}

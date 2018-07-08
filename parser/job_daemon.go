package parser

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

var (
	ErrBadJob  = errors.New("bad job request")
	ErrSkipJob = errors.New("skiped job")
)

func FloatRand(n float64) float64 {
	r := rand.Int63n(1 << 30)
	return float64(r) / (1 << 30) * n
}

func JobDaemon(pctx *Context) error {
	if err := initTorentClients(); err != nil {
		return errors.Wrap(err, "Failed initTorrentClients for parsing jobs")
	}
	go StartRutrackerSessionsGenerator()

	wg := &sync.WaitGroup{}
	maxJob := 50
	wg.Add(maxJob)
	for i := 0; i < maxJob; i++ {
		go func() {
			defer wg.Done()
			for {
				if err := NextJob(pctx); err != nil {
					if errors.Cause(err) != pgx.ErrNoRows {
						log.Printf("%+v\n", errors.Wrap(err, "failed NextJob()"))
					}
					time.Sleep(time.Second * time.Duration(rand.Intn(maxJob)+1))
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// if err := NextJobFetchTorrentPage(pctx); err != nil {
			// 	if errors.Cause(err) != pgx.ErrNoRows {
			// 		log.Printf("%+v\n", errors.Wrap(err, "failed NextJobFetchTorrentPage()"))
			// 	}
			// }
			// if err := NextJobFetchTorrentFile(pctx); err != nil {
			// 	if errors.Cause(err) != pgx.ErrNoRows {
			// 		log.Printf("%+v\n", errors.Wrap(err, "failed NextJobFetchTorrentFile()"))
			// 	}
			// }
			// if err := NextJobExtractFilesFromTorrentSource(pctx); err != nil {
			// 	if errors.Cause(err) != pgx.ErrNoRows {
			// 		log.Printf("%+v\n", errors.Wrap(err, "failed NextJobExtractFilesFromTorrentSource()"))
			// 	}
			// }
			if err := NextJobParseFiles(pctx); err != nil {
				if errors.Cause(err) != pgx.ErrNoRows {
					log.Printf("%+v\n", errors.Wrap(err, "failed NextJobParseFiles()"))
				}
			}
			time.Sleep(time.Second * 500)
		}
	}()
	wg.Wait()

	return nil
}

func NextJob(pctx *Context) (reterr error) {
	tx, err := pctx.Dbx.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	job := &JobRequest{}
	timestart := time.Now()
	if err = tx.QueryRow(`
	UPDATE gongo.parse_queue SET done = true
	WHERE id = (
		SELECT id FROM gongo.parse_queue
		WHERE
			defered<now() and
			tryed < 3 and
			done != true and
			job_type in ('PARSE_FILES')
		ORDER BY priority DESC, RANDOM()
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	)
	RETURNING
		id,
		job_type, 
		priority, 
		href, 
		"offset", 
		tracker_name, 
		ip, 
		tracker_torrent_id, 
		job_name, 
		torrent_id
	`).Scan(
		&job.ID,
		&job.JobType,
		&job.Priority,
		&job.HREF,
		&job.Offset,
		&job.TrackerName,
		&job.IP,
		&job.TrackerTorrentID,
		&job.JobName,
		&job.TorrentID,
	); err != nil {
		return errors.Wrap(err, "Failed query next job")
	}
	defer func(job *JobRequest, timestart time.Time) {
		r := recover()
		if r == nil {
			return
		}
		if rerr, ok := r.(error); ok {
			err = errors.Wrap(rerr, "Failed job")
		} else {
			err = errors.New("Failed job")
		}
		log.Printf("%+v\n", err)
		_, err = tx.Exec(`
			UPDATE
				gongo.parse_queue 
			SET done = false, defered = now() + interval '1 hour', tryed = tryed+1, duration=$2, log=$3::varchar||log
			WHERE id = $1
		`, *job.ID, time.Now().Sub(timestart).Seconds(), fmt.Sprintf("%+v", err))
		if err != nil {
			reterr = errors.Wrap(err, "Failed mark job as failed and defer to next try")
			return
		}
		reterr = tx.Commit()
	}(job, timestart)
	switch *job.JobType {
	case "PARSE_PAGE":
		err = JobParsePage(pctx, tx, job)
	case "EXTRACT_FILES_FROM_TORRENT":
		err = JobExtractFiles(pctx, tx, job)
	case "PARSE_FILES":
		err = JobParseFiles(pctx, tx, job)
	default:
		_, err = tx.Exec(`
			UPDATE
				gongo.parse_queue 
			SET done = false, defered = now() + interval '1 hour'
			WHERE id = $1
		`, *job.ID)
		if err != nil {
			return errors.Wrap(err, "Failed defer job")
		}
		return tx.Commit()
	}
	if errors.Cause(err) == ErrBadJob {
		log.Printf("%+v\n", errors.Wrap(err, "Failed job"))
		_, err = tx.Exec(`
			UPDATE
				gongo.parse_queue 
			SET done = true, tryed = 666, duration=$2, log = $3::varchar||log
			WHERE id = $1
		`, *job.ID, time.Now().Sub(timestart).Seconds(), fmt.Sprintf("%+v", err))
		if err != nil {
			return errors.Wrap(err, "Failed mark job as failed and defer to next try")
		}
		return tx.Commit()
	} else if err != nil {
		log.Printf("%+v\n", errors.Wrap(err, "Failed job"))
		_, err = tx.Exec(`
			UPDATE
				gongo.parse_queue 
			SET done = false, defered = now() + interval '1 hour', tryed = tryed+1, duration=$2, log=$3::varchar||log
			WHERE id = $1
		`, *job.ID, time.Now().Sub(timestart).Seconds(), fmt.Sprintf("%+v", err))
		if err != nil {
			return errors.Wrap(err, "Failed mark job as failed and defer to next try")
		}
		return tx.Commit()
	}
	_, err = tx.Exec(`
		UPDATE
			gongo.parse_queue
		SET done_time=now(), duration=$2, done = true
		WHERE id = $1
	`, *job.ID, time.Now().Sub(timestart).Seconds())
	if err != nil {
		return errors.Wrap(err, "Failed write success meta to done job")
	}
	return errors.Wrap(tx.Commit(), "Failed commit done job")
}

func NextJobFetchTorrentPage(pctx *Context) (reterr error) {
	tx, err := pctx.Dbx.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	count := ""
	if err = tx.QueryRow(`
		SELECT job_type FROM gongo.parse_queue
		WHERE
			job_type = 'PARSE_PAGE'	 and
			job_name = '/forum/viewtopic.php' and
			done = false and
			tryed < 3 and
			defered < now()
		LIMIT 1
	`).Scan(&count); err != nil && err != pgx.ErrNoRows {
		return errors.Wrap(err, "Failed check if need to add new jobs to fetch torrent pages")
	}
	if count != "" {
		return
	}
	// rows, err := tx.Query(`
	// 	SELECT tracker_torrent_id FROM gongo.torrent
	// 	WHERE
	// 		tryed_fetch_page < 3 and
	// 		content_html is null and
	// 		tracker_name = 'rutracker'
	// 	ORDER BY coalesce(seeders,0)+coalesce(leechers,0) DESC
	// 	LIMIT 100
	// `)
	rows, err := tx.Query(`
		SELECT tt.tracker_torrent_id FROM gongo.torrent tt
		LEFT JOIN gongo.parse_queue tq
		ON
			tq.tracker_torrent_id = tt.tracker_torrent_id and 
			tq.tracker_name = tt.tracker_name and
			tq.job_name = '/forum/viewtopic.php'
		WHERE
			tt.tryed_fetch_page < 3 and 
			tt.content_html is null and 
			tt.tracker_name = 'rutracker' and
			(tq.id is null or (tq.done = true and tq.add_time < now()- interval '7 day'))
		ORDER BY coalesce(tt.seeders,0)+coalesce(tt.leechers,0) DESC
		LIMIT 1000
	`)
	if err != nil {
		return errors.Wrap(err, "Failed read torrents without content_html")
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return errors.Wrap(err, "Failed read torrents without content_html")
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		url := ENV_RUTRACKER_HOST + "/forum/viewtopic.php?t=" + id
		if err := JobMaybeParsePage(tx, url, 0.1+float32(FloatRand(0.01)), false); err != nil {
			log.Printf("%+v\n", errors.Wrap(err, "Failed queue fetch torrent page"))
		}
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "Failed commit tx")
	}

	return nil
}

func NextJobFetchTorrentFile(pctx *Context) (reterr error) {
	tx, err := pctx.Dbx.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	count := ""
	if err = tx.QueryRow(`
		SELECT job_type FROM gongo.parse_queue
		WHERE
			job_type = 'PARSE_PAGE'	 and
			job_name = '/forum/dl.php' and
			done = false and
			tryed < 3 and
			defered < now()
		LIMIT 1
	`).Scan(&count); err != nil && err != pgx.ErrNoRows {
		return errors.Wrap(err, "Failed check if need to add new jobs to fetch torrent files")
	}
	if count != "" {
		return
	}
	rows, err := tx.Query(`
		SELECT tt.tracker_torrent_id FROM gongo.torrent tt
		LEFT JOIN gongo.parse_queue tq
		ON
			tq.tracker_torrent_id = tt.tracker_torrent_id and 
			tq.tracker_name = tt.tracker_name and
			tq.job_name = '/forum/dl.php'
		WHERE
			tt.tryed_fetch_torrent < 3 and 
			tt.torrent_source_file is null and 
			tt.tracker_name = 'rutracker' and
			(tq.id is null or (tq.done = true and tq.add_time < now()- interval '7 day'))
		ORDER BY coalesce(tt.seeders,0)+coalesce(tt.leechers,0) DESC
		LIMIT 1000
	`)
	if err != nil {
		return errors.Wrap(err, "Failed read torrents without torrent_source_file")
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return errors.Wrap(err, "Failed read torrents without torrent_source_file")
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		url := ENV_RUTRACKER_HOST + "/forum/dl.php?t=" + id
		if err := JobMaybeParsePage(tx, url, 0.1+float32(FloatRand(0.01)), false); err != nil {
			log.Printf("%+v\n", errors.Wrap(err, "Failed queue fetch torrent file"))
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "Failed commit tx")
	}

	return nil
}

func NextJobExtractFilesFromTorrentSource(pctx *Context) (reterr error) {
	tx, err := pctx.Dbx.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	count := ""
	if err = tx.QueryRow(`
		SELECT job_type FROM gongo.parse_queue
		WHERE
			job_type = 'EXTRACT_FILES_FROM_TORRENT'	 and
			done = false and
			tryed < 3 and
			defered < now()
		LIMIT 1
	`).Scan(&count); err != nil && err != pgx.ErrNoRows {
		return errors.Wrap(err, "Failed check if need to add new jobs to extract files")
	}
	if count != "" {
		return
	}
	rows, err := tx.Query(`
		SELECT tt.id FROM gongo.torrent tt
		LEFT JOIN gongo.parse_queue tq
		ON tq.torrent_id = tt.id and tq.job_type = 'EXTRACT_FILES_FROM_TORRENT'
		LEFT JOIN gongo.file tf
		ON tf.torrent_id = tt.id
		WHERE
			tt.torrent_source_file is not null and 
			tt.no_files != true and
			((tq.id is null and tf.torrent_id is null) or (tq.done = true and tq.add_time < now()- interval '7 day'))
		ORDER BY coalesce(tt.seeders,0)+coalesce(tt.leechers,0) DESC
		LIMIT 10000
	`)
	if err != nil {
		return errors.Wrap(err, "Failed read torrent_id`s to extract files")
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return errors.Wrap(err, "Failed read torrent_id`s to extract files")
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		_, err := tx.Exec(`
			INSERT INTO gongo.parse_queue
				(job_type, torrent_id, priority)
			VALUES
				('EXTRACT_FILES_FROM_TORRENT', $1, $2)
		`, id, 0.1+float32(FloatRand(0.01)))
		if err != nil {
			return errors.Wrap(err, "Failed insert job EXTRACT_FILES_FROM_TORRENT")
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "Failed commit tx")
	}

	return nil
}

func NextJobParseFiles(pctx *Context) (reterr error) {
	tx, err := pctx.Dbx.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	count := ""
	if err = tx.QueryRow(`
		SELECT job_type FROM gongo.parse_queue
		WHERE
			job_type = 'PARSE_FILES' and
			done = false and
			tryed < 3 and
			defered < now()
		LIMIT 1
	`).Scan(&count); err != nil && err != pgx.ErrNoRows {
		return errors.Wrap(err, "Failed check if need to add new jobs to parse files")
	}
	if count != "" {
		return
	}
	rows, err := tx.Query(`
		SELECT tt.id FROM gongo.torrent tt
		LEFT JOIN gongo.parse_queue tq
		ON tq.torrent_id = tt.id and tq.job_type = 'PARSE_FILES'
		LEFT JOIN gongo.file tf
		ON tf.torrent_id = tt.id
		WHERE
			tf.ext in ('cue') and
			tt.torrent_source_file is not null and
			tt.no_files != true and
			tf.parsed != true and
			(tq.id is null or (tq.done = true and tq.add_time < now()- interval '7 day'))
		ORDER BY coalesce(tt.seeders,0)+coalesce(tt.leechers,0) DESC
		LIMIT 1
	`)
	if err != nil {
		return errors.Wrap(err, "Failed read torrent_id`s to parse files")
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return errors.Wrap(err, "Failed read torrent_id`s to parse files")
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		_, err := tx.Exec(`
			INSERT INTO gongo.parse_queue
				(job_type, torrent_id, priority)
			VALUES
				('PARSE_FILES', $1, $2)
		`, id, 0.01+float32(FloatRand(0.001)))
		if err != nil {
			return errors.Wrap(err, "Failed insert job PARSE_FILES")
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "Failed commit tx")
	}

	return nil
}

// SELECT *
// FROM   (SELECT is_track,
//                ext,
//                Count(ext)                               AS count,
//                Round(Sum(size) / 1024 / 1024 / 1024, 3) AS sum_size_gb,
//                Round(Avg(size) / 1024 / 1024, 3)        AS avg_size_mb,
//                Round(Avg(Length(basename)))             AS avg_name_length
//         FROM   gongo.file
//         GROUP  BY ext,
//                   is_track) AS s1
// ORDER  BY s1.is_track,
//           s1.count DESC
// LIMIT  1000

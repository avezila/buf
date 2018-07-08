package parser

import (
	"bytes"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/saintfish/chardet"

	"github.com/djimenez/iconv-go"

	"github.com/AlekSi/pointer"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/avezila/gongo/torrent"
	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

var reIsTrack = regexp.MustCompile(`^(?:3gp|aac|aacp|aif|aiff|alac|ape|caf|flac|m4a|m4b|m4p|m4r|m4v|mka|mks|mp2|mp3|mp4|oga|ogg|ogv|ogx|opus|rmi|spx|tta|wav|wave|webm|wma|dsf|wv|dff|dts)$`)

func JobExtractFiles(pctx *Context, tx *pgx.Tx, req *JobRequest) error {

	tor := &torrent.Torrent{}
	if req.TorrentID == nil || *req.TorrentID == "" {
		return errors.New("TorrentID required for extract files from torrent")
	}
	if err := tx.QueryRow(
		`SELECT torrent_source_file FROM gongo.torrent WHERE id = $1`,
		*req.TorrentID,
	).Scan(&tor.TorrentSourceFile); err != nil || tor.TorrentSourceFile == nil {
		return errors.Wrap(err, "Failed read TorrentSourceFile for torrent "+*req.TorrentID)
	}
	if len(*tor.TorrentSourceFile) == 0 {
		if _, err := tx.Exec(`
			UPDATE gongo.torrent
			SET no_files = true
			WHERE id = $1
		`, *req.TorrentID); err != nil {
			return errors.Wrap(err, "Failed write no_files true for torrent "+*req.TorrentID)
		}
		return nil
	}
	meta, err := metainfo.Load(bytes.NewReader(*tor.TorrentSourceFile))
	if err != nil {
		log.Println("metainfo.Load() Bad torrent file in torrent " + *req.TorrentID)
		if _, err = tx.Exec(`
			UPDATE gongo.torrent
			SET no_files = true
			WHERE id = $1
		`, *req.TorrentID); err != nil {
			return errors.Wrap(err, "Failed write no_files true for torrent "+*req.TorrentID)
		}
		return nil
	}
	info, err := meta.UnmarshalInfo()
	if err != nil {
		log.Println("meta.UnmarshalInfo() Bad torrent file in torrent " + *req.TorrentID)
		if _, err := tx.Exec(`
			UPDATE gongo.torrent
			SET no_files = true
			WHERE id = $1
		`, *req.TorrentID); err != nil {
			return errors.Wrap(err, "Failed write no_files true for torrent "+*req.TorrentID)
		}
		return nil
	}
	wrote := 0
	for index, ifile := range info.Files {
		file := &torrent.File{}
		file.IndexInTorrent = pointer.ToInt32(int32(index))
		file.Size = pointer.ToInt64(ifile.Length)
		file.TorrentID = req.TorrentID
		displayPath := ifile.DisplayPath(&info)
		displayPath, err = ToUtf8(displayPath)
		if err != nil {
			log.Println(err)
			continue
		}
		displayPath = filepath.Clean(displayPath)
		file.Path = &displayPath
		err, n := JobExtractFilesWriteFile(tx, file)
		if err != nil {
			return err
		}
		wrote += n
	}
	if wrote == 0 {
		log.Println("No files in torrent " + *req.TorrentID)
		if _, err := tx.Exec(`
			UPDATE gongo.torrent
			SET no_files = true
			WHERE id = $1
		`, *req.TorrentID); err != nil {
			return errors.Wrap(err, "Failed write no_files true for torrent "+*req.TorrentID)
		}
		return nil
	}
	log.Printf("Extracted for %s %d", *req.TorrentID, wrote)
	return nil
}

func ToUtf8(str string) (string, error) {
	if utf8.ValidString(str) {
		return str, nil
	}
	for _, ch := range []string{"windows-1251", "KOI8-R", "KOI8-U", "KOI8-RU", "CP1251"} {
		str2, err := iconv.ConvertString(str, ch, "utf-8")
		if err == nil {
			log.Println("Converted from " + ch + "\n" + str + "\n" + str2)
			return str2, nil
		}
	}
	detector := chardet.NewTextDetector()
	res, err := detector.DetectBest([]byte(str))
	if err != nil {
		return "", errors.New("Failed detect charset for " + str)
	}
	newStr, err := iconv.ConvertString(str, res.Charset, "utf-8")
	if err != nil {
		return "", errors.New("Failed convert " + str + " from " + res.Charset)
	}
	log.Println("Converted from " + res.Charset + "\n" + str + "\n" + newStr)
	return newStr, nil
}

func JobExtractFilesWriteFile(tx *pgx.Tx, file *torrent.File) (error, int) {
	ext := strings.ToLower(filepath.Ext(*file.Path))
	if ext == "" {
		return nil, 0
	}
	ext = ext[1:]
	file.Ext = &ext
	base := filepath.Base(*file.Path)
	base = base[:len(base)-len(ext)-1]
	file.Basename = &base
	if reIsTrack.MatchString(ext) {
		file.IsTrack = pointer.ToBool(true)
	}
	query, args := file.Fields().NotNil().Upsert("gongo.file", "torrent_id", "index_in_torrent")
	if query == "" || args == nil {
		log.Printf("empty query for insert file %d %s %s\n", *file.IndexInTorrent, *file.TorrentID, *file.Path)
		return nil, 0
	}
	if _, err := tx.Exec(query, args...); err != nil {
		return errors.Wrap(err, "Failed insert file for torrent "+*file.TorrentID), 0
	}
	return nil, 1
}

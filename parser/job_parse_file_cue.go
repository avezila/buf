package parser

import (
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/vchimishuk/cue-go"

	atorrent "github.com/anacrolix/torrent"
	"github.com/avezila/gongo/torrent"
	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

var rePathStrip = regexp.MustCompile(`[^\p{L}\d\_\.\-\:\/\\]`)

func JobParseFileCue(pctx *Context, tx *pgx.Tx, req *JobRequest, ator *atorrent.Torrent, tor *torrent.Torrent, file *torrent.File) error {
	// log.Println(*file.IndexInTorrent)
	aindex := int(*file.IndexInTorrent)
	if aindex >= 1000000 || aindex < 0 {
		log.Println("No need to parse file")
		return nil
	}
	// log.Println(*file.IndexInTorrent)
	afiles := ator.Files()
	if aindex >= len(afiles) {
		log.Println("TorrentIndex out of range")
		return nil
	}
	// log.Println(*file.IndexInTorrent)
	afile := afiles[aindex]
	psc := ator.SubscribePieceStateChanges()
	defer psc.Close()
	afile.Download()
	done := false
_for1:
	for {
		// started := time.Now()
	_select1:
		select {
		case <-psc.Values:
		case <-time.After(time.Second):
			for _, p := range afile.State() {
				if !p.Complete {
					break _select1
				}
			}
			done = true
			break _for1
		case <-time.After(time.Hour):
			break _for1
		}
	}
	psc.Close()
	if !done {
		afile.Cancel()
		return errors.New("Timed out download cue file")
	}
	buf, err := ioutil.ReadFile(filepath.Join(TORRENTS_PATH_PARSE, ator.InfoHash().HexString(), afile.Path()))
	if err != nil {
		return errors.Wrap(err, "Faield read downloaded file")
	}
	// reader := ator.NewReader()
	// reader.SetResponsive()
	// reader.Seek(afile.Offset(), 0)
	// // log.Println(*file.IndexInTorrent)
	// readContext, _ := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
	// defer reader.Close()
	// go func() {
	// 	defer reader.Close()
	// 	select {
	// 	case <-time.After(time.Hour):
	// 		readCancel()
	// 	case <-readContext.Done():
	// 		return
	// 	}
	// }()
	// log.Println(*file.IndexInTorrent)
	// log.Println("len", afile.Length())
	// buf := make([]byte, afile.Length())
	// if _, err := reader.ReadContext(readContext, buf); err != nil {
	// 	return errors.Wrap(err, "Failed read cue file from torrent readseeker")
	// }
	// log.Println(*file.IndexInTorrent)
	utf8cue, err := ToUtf8(string(buf))
	if err != nil {
		log.Println("failed convert cue to utf8", string(buf))
		return nil
	}
	sheet, err := cue.Parse(strings.NewReader(utf8cue))
	if err != nil {
		log.Println("Failed parse cue file", utf8cue)
		return nil
	}
	cuepath := afile.Path()
	if cuepath, err = ToUtf8(cuepath); err != nil {
		log.Println("%s %+v\n", afile.Path(), errors.New("Failed compile cue path to utf8"))
		return nil
	}
	cuepath = rePathStrip.ReplaceAllString(path.Clean(cuepath), "")
	cuedir := path.Dir(cuepath)
	for indexPart, sfile := range sheet.Files {
		// nfile := &torrent.File{}

		spath := path.Clean(rePathStrip.ReplaceAllString(sfile.Name, ""))
		if path.IsAbs(spath) {
			spath = path.Base(spath)
		}
		fullspath := path.Join(cuedir, spath)
		var gotIndex = -1
		for index, tfile := range ator.Files() {
			checkpath := tfile.Path()
			if checkpath, err = ToUtf8(checkpath); err != nil {
				continue
			}
			checkpath = rePathStrip.ReplaceAllString(path.Clean(checkpath), "")
			if fullspath == checkpath {
				gotIndex = index
				break
			}
		}
		if gotIndex < 0 {
			log.Println("Path in cue file not found", sfile.Name)
			continue
		}

		for indexTrack, strack := range sfile.Tracks {
			nfile := &torrent.File{}

			nfile.IndexInTorrent = pointer.ToInt32(int32(gotIndex*1000000 + indexTrack))
			nfile.MetaPartPosition = pointer.ToInt32(int32(indexPart + 1))
			nfile.MetaPartPositionTotal = pointer.ToInt32(int32(len(sheet.Files)))
			nfile.MetaBarCode = pointer.ToString(sheet.Catalog)
			nfile.MetaCDTextFile = pointer.ToString(sheet.CdTextFile)
			log.Println("sheet.Comments", sheet.Comments)
			nfile.MetaPerformer = pointer.ToString(sheet.Performer)
			nfile.MetaSongWriter = pointer.ToString(sheet.Songwriter)
			nfile.MetaAlbum = pointer.ToString(sheet.Title)
			for _, ind := range strack.Indexes {
				if ind.Number == 0 {
					nfile.MetaDelay = pointer.ToInt64(int64((ind.Time.Min*60+ind.Time.Sec)*1000) + int64(float32(ind.Time.Frames)*1000/75))
				} else if ind.Number == 1 {
					nfile.AudioOffsetMS = pointer.ToInt64(int64((ind.Time.Min*60+ind.Time.Sec)*1000) + int64(float32(ind.Time.Frames)*1000/75))
				}
			}
			nfile.MetaISRC = pointer.ToString(strack.Isrc)
			nfile.MetaTrackPosition = pointer.ToInt32(int32(strack.Number))
			nfile.MetaTrackPositionTotal = pointer.ToInt32(int32(len(sfile.Tracks)))
			if strack.Performer != "" {
				nfile.MetaPerformer = pointer.ToString(strack.Performer)
			}
			if strack.Postgap.Frames != 0 || strack.Postgap.Min != 0 || strack.Postgap.Sec != 0 {
				nfile.MetaPostgap = pointer.ToInt64(int64((strack.Postgap.Min*60+strack.Postgap.Sec)*1000) + int64(float32(strack.Postgap.Frames)*1000/75))
			}
			if strack.Pregap.Frames != 0 || strack.Pregap.Min != 0 || strack.Pregap.Sec != 0 {
				nfile.MetaPregap = pointer.ToInt64(int64((strack.Pregap.Min*60+strack.Pregap.Sec)*1000) + int64(float32(strack.Pregap.Frames)*1000/75))
			}
			if strack.Songwriter != "" {
				nfile.MetaSongWriter = pointer.ToString(strack.Songwriter)
			}
			nfile.MetaTrack = pointer.ToString(strack.Title)
			log.Printf("%+v\n", nfile.Fields().NotNil())
		}
	}
	// log.Println(*file.IndexInTorrent)

	log.Println("got cue", len(utf8cue))
	return nil
}

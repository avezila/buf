package parser

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"

	"github.com/avezila/gongo/torrent"
	"github.com/jackc/pgx"
	"github.com/pkg/errors"
	"github.com/segmentio/go-snakecase"
)

func JobParseFiles(pctx *Context, tx *pgx.Tx, req *JobRequest) error {
	tor := &torrent.Torrent{}
	files := []*torrent.File{}
	if err := tx.QueryRow(`
		SELECT 
			torrent_source_file, id
		FROM gongo.torrent WHERE id = $1
	`, *req.TorrentID).Scan(&tor.TorrentSourceFile, &tor.ID); err != nil {
		return errors.Wrap(err, "Failed select torrents to parse files")
	}
	rows, err := tx.Query(`
		SELECT
			id, index_in_torrent, ext, path
		FROM gongo.file
		WHERE
			torrent_id = $1 and
			ext in ('cue') and
			parsed != true
	`, *tor.ID)
	if err != nil {
		return errors.Wrap(err, "Failed select files to parse where torrent.id "+*tor.ID)
	}
	defer rows.Close()
	for rows.Next() {
		file := &torrent.File{}
		err = rows.Scan(&file.ID, &file.IndexInTorrent, &file.Ext, &file.Path)
		if err != nil {
			return errors.Wrap(err, "Failed scan file to parse")
		}
		files = append(files, file)
	}
	if rows.Err() != nil {
		return errors.Wrap(err, "failed scan files rows to parse")
	}
	rows.Close()

	meta, err := metainfo.Load(bytes.NewReader(*tor.TorrentSourceFile))
	if err != nil {
		return errors.Wrap(err, "Failed read metainfo from torrentsourcefile")
	}

	ator, err := torParseClient.AddTorrent(meta)
	if err != nil {
		return errors.Wrap(err, "Failed add torrent from meta")
	}
	hash := ator.InfoHash().HexString()
	defer os.RemoveAll(filepath.Join(TORRENTS_PATH_PARSE, hash))
	defer ator.Drop()
	// log.Println("before", ator.Metainfo().AnnounceList)
	atrackers := [][]string{}
	for _, f := range ator.Metainfo().AnnounceList {
		atrackers = append(atrackers, f)
	}
	for _, f := range AnonceTrackers {
		atrackers = append(atrackers, f)
	}
	ator.AddTrackers(atrackers)
	// log.Println("after", ator.Metainfo().AnnounceList)
	<-ator.GotInfo()
	log.Println("plen", ator.Piece(0).Info().Length())
	// log.Println("got info", ator.Files()[0].DisplayPath())
	// ator.DownloadAll()
	// for {
	// 	log.Println("%+v\n", ator.Stats())
	// 	time.Sleep(time.Second)
	// }
	wg := &sync.WaitGroup{}
	errs := make(chan error, len(files))
	wg.Add(len(files))
	for _, file := range files {
		go func(file *torrent.File) {
			defer wg.Done()
			if err := JobParseFileCue(pctx, tx, req, ator, tor, file); err != nil {
				errs <- errors.Wrap(err, "failed parse cue file")
			}
		}(file)
	}
	wg.Wait()
	close(errs)
	err, _ = <-errs
	for err := range errs {
		log.Printf("%+v\n", err)
	}
	if err != nil {
		return err
	}
	log.Println("ok")
	time.Sleep(time.Second * 60)
	tx.Rollback()
	return nil
}

type MediaInfoSection int

const (
	General MediaInfoSection = 0
	Audio                    = 1
)

type MetaTag struct {
	Section     MediaInfoSection
	Type        interface{}
	SQL         string
	MediaInfo   string
	Matroska    string
	Vorbis      string
	APEv2       string
	Description string
}

var MetaTags = []MetaTag{
	{0, "", "", "Track", "", "TITLE", "TITLE", "Title of the track or chapter"},
	{0, "", "", "Track_More", "", "VERSION", "SUBTITLE", "Subtitle. May also be used to differentiate multiple versions of the same tracktitle in a single collection"},
	{0, int32(0), "", "Track/Position", "", "TRACKNUMBER", "", "Number of the current track"},
	{0, "", "", "Track/Url", "", "", "", ""},
	{0, int32(0), "", "Track/Position_Total", "", "", "", ""},
	{0, "", "", "Part", "", "", "", "Name of the part. e.g. CD1, CD2"},
	{0, int32(0), "", "Part/Position", "", "DISCNUMBER", "", "Number of the current part. Usualy the number in a multi-disc album"},
	{0, int32(0), "", "Part/Position_Total", "", "", "", ""},
	{0, "", "", "Album", "", "ALBUM", "ALBUM", "Title of the album"},
	{0, "", "", "Album_More", "", "", "", ""},
	{0, "", "", "Chapter", "", "", "", "Name of the chapter."},
	{0, "", "", "SubTrack", "", "", "", "Name of the subtrack."},
	{0, "", "", "Performer", "LEAD_PERFORMER", "", "ARTIST", "A person or band/collective generally considered responsible for the work: Singer, Realisator"},
	{0, "", "", "Performer/Url", "", "", "", "Official artist/performer webpage"},
	{0, "", "", "Original/Performer", "", "", "", "Original artist(s)/performer(s)."},
	{0, "", "", "Accompaniment", "", "ENSEMBLE", "", "Band/orchestra/accompaniment/musician"},
	{0, "", "", "SongWriter", "", "", "", ""},
	{0, "", "", "Composer", "", "", "", "Name of the original composer"},
	{0, "", "", "Composer/Nationality", "COMPOSER_NATIONALITY", "", "", "Nationality of the main composer of the item, mostly for classical music."},
	{0, "", "", "Arranger", "", "", "", "The person who arranged the piece"},
	{0, "", "", "Lyricist", "", "", "", "The person who wrote the lyrics for a musical item."},
	{0, "", "", "Conductor", "", "", "", "The artist(s) who performed the work. In classical music this would be the conductor, orchestra, soloists."},
	{0, "", "", "SoundEngineer", "", "", "", "The name of the sound engineer or sound recordist."},
	{0, "", "", "MasteredBy", "MASTERED_BY", "", "", "The engineer who mastered the content for a physical medium or for digital distribution."},
	{0, "", "", "RemixedBy", "REMIXED_BY", "", "", "Interpreted, remixed, or otherwise modified by."},
	{0, "", "", "Label", "", "ORGANIZATION", "", "The record label or imprint on the disc."},
	{0, "", "", "Publisher", "", "", "", "Name of the organization producing the track (i.e. the 'record label')."},
	{0, "", "", "DistributedBy", "", "", "", ""},
	{0, "", "", "RadioStation", "", "", "", ""},
	{0, "", "", "Subject", "", "", "", `Describes the topic of the file, such as "Aerial view of Seattle.".`},
	{0, "", "", "Description", "", "", "", `A short description of the contents, such as "Two birds flying".`},
	{0, "", "", "Keywords", "", "", "", "Keywords to the item separated by a comma, used for searching."},
	{0, "", "", "Period", "", "", "", `Describes the period that the piece is from or about; e.g. "Renaissance".`},
	{0, "", "", "LawRating", "LAW_RATING", "", "", "Depending on the country it's the format of the rating of a movie (P, R, X in the USA, an age in other countries or a URI defining a logo)."},
	{0, "", "", "LawRating_Reason", "", "", "", ""},
	{0, "", "", "ICRA", "", "", "", "The ICRA rating. (Previously RSACi)"},
	{0, "", "", "Language", "TagLanguage", "", "", "Language(s) of the item in the bibliographic ISO-639-2 form."},
	{0, "", "", "Country", "", "", "", ""},
	{0, "", "", "Written_Date", "DATE_WRITTEN", "", "", "The time that the composition of the music/script began."},
	{0, "", "", "Recorded_Date", "DATE_RECORDED", "", "RECORD DATE", "time/date/year The time that the recording began."},
	{0, "", "", "Released_Date", "DATE_RELEASED", "DATE", "YEAR", "The time that the item was originaly released."},
	{0, "", "", "Mastered_Date", "", "", "", "time/date/year The time that the item was tranfered to a digitalmedium."},
	{0, "", "", "Written_Location", "COMPOSITION_LOCATION", "", "", `Location that the item was originaly designed/written. Information should be stored in the following format: "country code, state/province, city" where the coutry code is the same 2 octets as in Internet domains, or possibly ISO-3166; e.g. "US, Texas, Austin" or "US, , Austin".`},
	{0, "", "", "Recorded_Location", "RECORDING_LOCATION", "LOCATION", "RECORD LOCATION", "Location where track was recorded. (See COMPOSITION_LOCATION for format)"},
	{0, "", "", "Genre", "", "", "", `The main genre of the audio or video; e.g. "classical", "ambient-house", "synthpop", "sci-fi", "drama", etc.`},
	{0, "", "", "Mood", "", "", "", `Intended to reflect the mood of the item with a few keywords, e.g. "Romantic", "Sad", "Uplifting", etc.`},
	{0, "", "", "Comment", "", "", "", "Any comment related to the content"},
	{0, "", "", "Rating", "", "", "", "A numeric value defining how much a person likes the song/movie. The number is between 0 and 5 with decimal values possible (e.g. 2.7), 5(.0) being the highest possible rating."},
	{0, "", "", "EncodedBy", "", "", "", ""},
	{0, "", "", "Encoded_Date", "", "", "", ""},
	{0, "", "", "Encoded_Original", "ORIGINAL_MEDIA_TYPE", "SOURCEMEDIA", "", `Identifies the original recording media form from which the material originated, such as "CD", "cassette", "LP", "radio broadcast", "slide", "paper", etc.`},
	{0, "", "", "Encoded_Original/Url", "", "", "", "Official audio source webpage; e.g. a movie."},
	{0, "", "", "FileName_Original", "", "", "", "Contains the preferred filename for the file"},
	{0, "", "", "File_Url", "", "", "", ""},
	{0, "", "", "Lyrics", "", "", "", "Text of a song"},
	{0, float32(0), "", "BPS", "", "", "", "The average bits per second of the specified item."},
	{0, float32(0), "", "BPM", "", "", "", "The average bits per minute of the specified item."},
	{0, int64(0), "", "Duration", "", "", "", "Length of the audio file in milliseconds."},
	{0, int64(0), "", "Delay", "", "", "", "Defines the numbers of milliseconds of silence that should be inserted before this audio."},
	{0, "", "", "Album_ReplayGain_Gain", "", "", "", "The gain to apply to reach 89db SPL on playback. Based on the Replay Gain standard. Note that ReplayGain information can be found at all TargetType levels (track, album, etc)."},
	{0, "", "", "Album_ReplayGain_Peak", "REPLAYGAIN_PEAK", "", "", "The maximum absolute peak value of the item. Based on the Replay Gain standard."},
	{0, "", "", "Purchase_Info", "PURCHASE_INFO", "", "", "Commercial information about this item."},
	{0, "", "", "Purchase_Price", "PURCHASE_PRICE", "", "", `The amount paid for entity. There should only be a numeric value in here; e.g. "15.59" instead of "$15.59USD".`},
	{0, "", "", "Purchase_Currency", "", "", "", "The ISO-4217 currency type used to pay for the entity."},
	{0, "", "", "Purchase_Item", "", "", "", "URL to purchase this item."},
	{0, "", "", "Copyright", "", "", "", "Copyright attribution."},
	{0, "", "", "Producer_Copyright", "PRODUCTION_COPYRIGHT", "", "PUBLICATIONRIGHT", "	The copyright information as per the productioncopyright holder."},
	{0, "", "", "TermsOfUse", "TERMS_OF_USE", "LICENSE", "", `License information, e.g., "All Rights Reserved","Any Use Permitted".`},
	{0, "", "", "Copyright/Url", "", "", "", "Copyright/legal information."},
	{0, "", "", "ISRC", "", "", "", `International Standard Recording Code, excluding the "ISRC" prefix and including hyphens.`},
	{0, "", "", "MSDI", "MCDI", "", "", "This is a binary dump of the TOC of the CDROM that this item was taken from."},
	{0, "", "", "ISBN", "", "", "", "International Standard Book Number."},
	{0, "", "", "BarCode", "upc_ean", "EAN/UPN", "EAN/UPC", "EAN-13 (13-digit European Article Numbering) or UPC-A (12-digit Universal Product Code) bar code identifier."},
	{0, "", "", "LCCN", "", "", "", "Library of Congress Control Number."},
	{0, "", "", "CatalogNumber", "CATALOG_NUMBER", "LABELNO", "CATALOG", `A label-specific catalogue number used to identify the release; e.g. "TIC 01".`},
	{0, "", "", "LabelCode", "LABEL_CODE", "", "LC", "A 4-digit or 5-digit number to identify the record label, typically printed as (LC) xxxx or (LC) 0xxxx on CDs medias or covers, with only the number being stored."},

	{0, "", "container", "Format", "", "", "", "For ex. MPEG-4"},
	{1, "", "", "ReplayGain_Gain", "", "", "", "The gain to apply to reach 89db SPL on playback. Based on the Replay Gain standard. Note that ReplayGain information can be found at all TargetType levels (track, album, etc)."},
	{1, "", "", "ReplayGain_Peak", "REPLAYGAIN_PEAK", "", "", "The maximum absolute peak value of the item. Based on the Replay Gain standard."},
	{1, "", "codec", "Format", "", "", "", "AAC"},
	{1, "", "", "Format_Compression", "", "", "", "Lossy"},
	{1, "", "", "Compression_Mode", "", "", "", "Lossy"},
	{1, "", "", "BitRate_Mode", "", "", "", "VBR, CBR"},
	{1, float32(0), "", "BitRate", "", "", "", ""},
	{1, float32(0), "", "BitRate_Minimum", "", "", "", ""},
	{1, float32(0), "", "BitRate_Nominal", "", "", "", ""},
	{1, float32(0), "", "BitRate_Maximum", "", "", "", ""},
	{1, int32(0), "", "Channels", "", "", "", ""},
	{1, "", "", "Channels_Original", "", "", "", ""},
	{1, "", "", "ChannelPositions", "", "", "", ""},
	{1, float64(0), "", "SamplesPerFrame", "", "", "", ""},
	{1, int64(0), "", "SamplingRate", "", "", "", ""},
	{1, float64(0), "", "FrameRate", "", "", "", ""},
	{1, int32(0), "", "FrameCount", "", "", "", ""},
	{1, int32(0), "", "SamplingCount", "", "", "", ""},

	{1, int32(0), "", "BitDepth", "", "", "", ""},

	{0, int32(0), "", "HeaderSize", "", "", "", ""},
	{0, int64(0), "", "DataSize", "", "", "", ""},
	{0, "", "", "IsStreamable", "", "", "", ""},
	{0, "", "", "DISCID", "", "", "", ""},

	{0, "", "", "AppleStoreCatalogID", "", "", "", ""},
	{0, "", "", "AlbumTitleID", "", "", "", ""},
	{0, "", "", "AppleStoreCountry", "", "", "", ""},
	{0, "", "", "AppleStoreAccountType", "", "", "", ""},
	{0, "", "", "AppleStoreAccount", "", "", "", ""},
	{0, "", "", "ContentType", "", "", "", ""},
	{0, "", "", "InternetMediaType", "", "", "", ""},
	{0, "", "", "PurchaseDate", "", "", "", ""},
	{0, "", "", "PlayListID", "", "", "", ""},
	{0, "", "", "GenreID", "", "", "", ""},
	{0, "", "", "Vendor", "", "", "", ""},
	{0, "", "", "OverallBitRate", "", "", "", ""},
	{0, "", "", "cmID", "", "", "", ""},
	{0, "", "", "Flavour", "", "", "", ""},

	{0, "", "cd_text_file", "CDTextFile", "", "", "", ""},
	{0, "", "", "PREGAP", "", "", "", ""},
	{0, "", "", "POSTGAP", "", "", "", ""},
}

var MediainfoByTag = map[string]*MetaTag{}

func init() {
	for _, row := range MetaTags {
		tag := row
		if tag.SQL == "" {
			tag.SQL = snakecase.Snakecase(tag.MediaInfo)
		}
		for _, s := range []string{tag.MediaInfo, tag.Matroska, tag.Vorbis, tag.APEv2} {
			if s == "" {
				continue
			}
			s = snakecase.Snakecase(s)
			if _, ok := MediainfoByTag[s]; !ok {
				MediainfoByTag[s] = &tag
			}
		}
		log.Println(tag.SQL)
	}
}

func ExtractTags(str string) (_tag string, _value string) {
	snaked := snakecase.Snakecase(str)
	lower := strings.ToLower(str)
	for key, tag := range MediainfoByTag {
		if len(snaked) < len(key) {
			continue
		}
		if snaked[:len(key)] != key {
			continue
		}
		ks := strings.Split(key, "_")
		for _, sk := range ks {
			i := strings.Index(lower, sk)
			if i > 0 {
				lower = lower[i+len(sk):]
				str = str[i+len(sk):]
			}
		}
		str = strings.TrimSpace(str)
		return tag.SQL, str
	}
	return "", ""
}

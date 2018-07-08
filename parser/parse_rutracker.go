package parser

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	uber_atomic "go.uber.org/atomic"

	"github.com/AlekSi/pointer"
	"github.com/goware/urlx"
	"go.uber.org/ratelimit"

	"github.com/avezila/gongo/torrent"

	"github.com/PuerkitoBio/goquery"
	iconv "github.com/djimenez/iconv-go"

	"github.com/pkg/errors"

	"github.com/jackc/pgx"
)

var RutrackerLimiter = ratelimit.New(200)
var RutrackerFailLimiter = ratelimit.New(10)

func ParseRutracker(pctx *Context, tx *pgx.Tx, req *JobRequest, u *url.URL) error {
	RutrackerLimiter.Take()
	var err error = nil
	if u.Path == "/forum/viewforum.php" {
		err = ParseRutrackerForum(tx, req, u)
	} else if u.RequestURI() == "/forum/index.php?map=" {
		err = ParseRutrackerForumMap(tx, req, u)
	} else if u.Path == "/forum/viewtopic.php" {
		err = ParseRutrackerTopick(tx, req, u)
	} else if u.Path == "/forum/dl.php" {
		err = ParseRutrackerTorrentFile(pctx, tx, req, u)
	} else {
		return errors.Wrap(ErrSkipJob, "no info how to parse this url")
	}
	if err != nil {
		RutrackerFailLimiter.Take()
	}
	return err
}

var reRutrackerDLPHP = regexp.MustCompile(`(?im)(dl\.php\?t=\d+)`)
var reNotD = regexp.MustCompile(`\D`)
var throttleDecode = time.Tick(time.Second * 10)
var rutrackerSession = uber_atomic.NewString("")

type HTMLForumTr struct {
	Title       string
	Seeders     int
	Leechers    int
	TorrentId   string
	ForumId     string
	Size        int64
	Downloaded  int
	Commented   int
	CreatorId   string
	CreatorName string
}

var reAudio = regexp.MustCompile(`(?ims)audio|аудио|music|музык|оцифровки|Hi-Res`)

func ParseRutrackerForumMap(tx *pgx.Tx, req *JobRequest, u *url.URL) error {
	client := &http.Client{}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	res, err := client.Get(u.String())
	if err != nil {
		return errors.Wrap(err, "Failed request this url")
	}
	defer res.Body.Close()
	if res.ContentLength > 1024*1024*100 {
		return errors.New("Too big response for html page")
	}
	utfBody, err := iconv.NewReader(res.Body, "Windows-1251", "utf-8")
	if err != nil {
		return errors.Wrap(err, "Failed create iconv.NewReader for Windows-1251")
	}
	doc, err := goquery.NewDocumentFromReader(utfBody)
	if err != nil {
		return errors.Wrap(err, "Failed create document from html response")
	}
	links := []*goquery.Selection{}
	doc.Find(".tree-root").Each(func(i int, treeroot *goquery.Selection) {
		title, _ := treeroot.Find("span.b>span.c-title").Attr("title")
		if reAudio.MatchString(title) {
			treeroot.Find("li>ul>li>ul>li>span>a").Each(func(i int, link *goquery.Selection) {
				log.Println("found", link.Text())
				links = append(links, link)
			})
		} else {
			treeroot.Find("li>ul>li").Each(func(i int, treesubroot *goquery.Selection) {
				if reAudio.MatchString(treesubroot.Find("span.b>a").Text()) {
					treesubroot.Find("ul>li>span>a").Each(func(i int, link *goquery.Selection) {
						log.Println("found", link.Text())
						links = append(links, link)
					})
				} else {
					treesubroot.Find("ul>li>span>a").Each(func(i int, link *goquery.Selection) {
						if reAudio.MatchString(link.Text()) {
							log.Println("found", link.Text())
							links = append(links, link)
						}
					})
				}
			})
		}
	})
	for _, link := range links {
		href, ok := link.Attr("href")
		if regexp.MustCompile(`\d+`).MatchString(href) {
			href = "viewforum.php?f=" + href
		}
		if !ok || !strings.Contains(href, "viewforum.php") {
			continue
		}
		uhref, err := urlx.Parse(href)
		if err != nil {
			log.Printf("%+v\n", errors.Wrap(err, "Failed parse rutracker forum href in sitemap"))
			continue
		}
		uhref = u.ResolveReference(uhref)
		if err := JobMaybeParsePage(tx, uhref.String(), *req.Priority, false); err != nil {
			log.Printf("%+v\n", errors.Wrap(err, "Failed queue rutracker forum page from sitemap"))
		}
	}
	return nil
}

func ParseRutrackerForum(tx *pgx.Tx, req *JobRequest, u *url.URL) error {
	client := &http.Client{}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	res, err := client.Get(u.String())
	if err != nil {
		return errors.Wrap(err, "Failed request this url")
	}
	defer res.Body.Close()
	if res.ContentLength > 1024*1024*100 {
		return errors.New("Too big response for html page")
	}
	utfBody, err := iconv.NewReader(res.Body, "Windows-1251", "utf-8")
	if err != nil {
		return errors.Wrap(err, "Failed create iconv.NewReader for Windows-1251")
	}
	doc, err := goquery.NewDocumentFromReader(utfBody)
	if err != nil {
		return errors.Wrap(err, "Failed create document from html response")
	}
	forumTitle := doc.Find(".maintitle a").Text()
	errslog := []string{}
	foundSmth := false
	doc.Find("#main_content tr.hl-tr[id][data-topic_id]").Each(func(i int, s *goquery.Selection) {
		log.Println(s.Find("a.torTopic").Text())
		tr := &HTMLForumTr{}
		torTopic := s.Find("a.torTopic")
		tr.Title = torTopic.Text()
		href, ok := torTopic.Attr("href")
		if !ok {
			return
		}
		if uhref, err := urlx.Parse(href); err != nil {
			return
		} else {
			tr.TorrentId = uhref.Query().Get("t")
		}
		if tr.TorrentId == "" {
			return
		}
		creator := s.Find("a.topicAuthor")
		if href, ok := creator.Attr("href"); ok {
			if uhref, err := urlx.Parse(href); err == nil {
				tr.CreatorId = uhref.Query().Get("u")
			}
		}
		tr.CreatorName = creator.Text()

		tr.Seeders, _ = strconv.Atoi(
			string(reNotD.ReplaceAll([]byte(
				s.Find(".vf-col-tor span[title='Seeders'] b").Text(),
			), nil)),
		)
		tr.Leechers, _ = strconv.Atoi(
			string(reNotD.ReplaceAll([]byte(
				s.Find(".vf-col-tor span[title='Leechers'] b").Text(),
			), nil)),
		)

		size := s.Find(".vf-col-tor a.f-dl").Text()

		if found := regexp.MustCompile(`([\d.]+).*(\w{2})`).FindStringSubmatch(size); len(found) == 3 {
			size, err := strconv.ParseFloat(found[1], 64)
			if err == nil {
				switch strings.ToLower(found[2]) {
				case "kb":
					size *= 1 << 10
				case "mb":
					size *= 1 << 20
				case "gb":
					size *= 1 << 30
				case "tb":
					size *= 1 << 40
				case "pb":
					size *= 1 << 50
				}
				tr.Size = int64(size)
			}
		}
		tr.Downloaded, _ = strconv.Atoi(
			string(reNotD.ReplaceAll([]byte(
				s.Find(".vf-col-replies span[title='Ответов']").Text(),
			), nil)),
		)
		tr.Commented, _ = strconv.Atoi(
			string(reNotD.ReplaceAll([]byte(
				s.Find(".vf-col-replies p[title='Торрент скачан'] b").Text(),
			), nil)),
		)
		foundSmth = true
		tor := &torrent.Torrent{}
		tor.TrackerName = pointer.ToString("rutracker")
		tor.TrackerTorrentID = pointer.ToString(tr.TorrentId)
		tor.Size = pointer.ToInt64(tr.Size)
		tor.Title = pointer.ToString(tr.Title)
		tor.ForumID = pointer.ToString(u.Query().Get("f"))
		tor.ForumTitle = pointer.ToString(forumTitle)
		tor.CreatorUsername = pointer.ToString(tr.CreatorName)
		tor.CreatorID = pointer.ToString(tr.CreatorId)
		tor.Seeders = pointer.ToInt32(int32(tr.Seeders))
		tor.Leechers = pointer.ToInt32(int32(tr.Leechers))
		tor.RepliesCount = pointer.ToInt32(int32(tr.Commented))
		tor.DownloadedCount = pointer.ToInt32(int32(tr.Downloaded))

		query, args := tor.Fields().NotNil().Upsert("gongo.torrent", "tracker_name", "tracker_torrent_id")
		if query == "" || args == nil {
			return
		}
		if _, err := tx.Exec(query, args...); err != nil {
			errslog = append(errslog, errors.Wrapf(err, "Failed upsert torrent with id=%s", tor.TrackerTorrentID).Error())
		}
	})

	if errslog != nil {
		if _, err := tx.Exec(`UPDATE gongo.parse_queue SET log=$2||log WHERE id=$1`, req.ID, errslog); err != nil {
			log.Printf("%+v\n", errors.Wrap(err, "Failed add log to job"))
		}
	}
	if foundSmth {
		qstart := u.Query().Get("start")
		offset := 0
		if qstart != "" {
			offset, err = strconv.Atoi(qstart)
			if err != nil {
				log.Printf("%+v\n", errors.Wrap(err, "failed parse forum start param"))
				offset = -1
			}
		}
		if offset >= 0 {
			nu := *u
			q := nu.Query()
			q.Set("start", strconv.Itoa(offset+50))
			nu.RawQuery = q.Encode()

			if err := JobMaybeParsePage(tx, nu.String(), *req.Priority, false); err != nil {
				log.Printf("%+v\n", errors.Wrap(err, "Failed queue next rutracker forum page"))
			}
		}
	}
	return nil
}

func ParseRutrackerTopick(tx *pgx.Tx, req *JobRequest, u *url.URL) error {
	client := &http.Client{}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	res, err := client.Get(u.String())
	if err != nil {
		return errors.Wrap(err, "Failed request this url "+u.String())
	}
	defer res.Body.Close()
	if res.ContentLength > 1024*1024*100 {
		return errors.New("Too big response for html page " + u.String())
	}
	utfBody, err := iconv.NewReader(res.Body, "Windows-1251", "utf-8")
	if err != nil {
		return errors.Wrap(err, "Failed create iconv.NewReader for Windows-1251 "+u.String())
	}
	doc, err := goquery.NewDocumentFromReader(utfBody)
	if err != nil {
		return errors.Wrap(err, "Failed create document from html response "+u.String())
	}
	tor := &torrent.Torrent{}
	tor.Title = pointer.ToString(doc.Find("a#topic-title").Text())
	magnet := doc.Find("a.magnet-link")
	magnetURI, _ := magnet.Attr("href")
	tor.MagnetURI = pointer.ToString(magnetURI)
	tor.TrackerName = pointer.ToString("rutracker")
	tor.TrackerTorrentID = pointer.ToString(u.Query().Get("t"))
	body := doc.Find(".post_body").First()
	if bodyhtml, err := body.Html(); err != nil {
		log.Printf("%s %+v\n", u.String(), errors.Wrap(err, "Failed read body.html()"))
	} else {
		tor.ContentHTML = pointer.ToString(bodyhtml)
	}
	tor.ContentText = pointer.ToString(body.Text())
	forumA := doc.Find("table.w100>tbody>tr>td>a").Last()
	forumLink, _ := forumA.Attr("href")
	if forumLinkU, err := urlx.Parse(forumLink); err == nil {
		tor.ForumID = pointer.ToString(forumLinkU.Query().Get("f"))
		tor.ForumTitle = pointer.ToString(forumA.Text())
	}
	tor.CreatorUsername = pointer.ToString(doc.Find(".nick-author").First().Text())
	creatorA := doc.Find("td.poster_btn>div>a.txtb").First()
	creatorLink, _ := creatorA.Attr("href")
	if creatorLinkU, err := urlx.Parse(creatorLink); err == nil {
		tor.CreatorID = pointer.ToString(creatorLinkU.Query().Get("u"))
	}
	if *tor.TrackerTorrentID == "" || *tor.Title == "" {
		log.Println(u.String(), regexp.MustCompile(`\s\s+`).ReplaceAllString(doc.Find("#main_content").First().Text(), " "))
		return errors.New("torrent page not found parsing this href " + u.String())
	}
	if *tor.ContentText == "" {
		log.Println(u.String(), regexp.MustCompile(`\s\s+`).ReplaceAllString(doc.Find("#main_content").First().Text(), " "))
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_page = tryed_fetch_page+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.New("torrent page not found parsing this href")
	}
	query, args := tor.Fields().NotNil().Upsert("gongo.torrent", "tracker_name", "tracker_torrent_id")
	if query == "" || args == nil {
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_page = tryed_fetch_page+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.New("fetch torrent page empty upsert torrent query")
	}
	if _, err := tx.Exec(query, args...); err != nil {
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_page = tryed_fetch_page+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.Wrapf(err, "Failed upsert torrent with id=%s", tor.TrackerTorrentID)
	}
	return nil
}

func ParseRutrackerTorrentFile(pctx *Context, tx *pgx.Tx, req *JobRequest, u *url.URL) error {
	client := &http.Client{}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	httpreq, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrap(err, "Failed http.NewRequest this url")
	}
	rusession := ""
	for rusession == "" {
		rusession = <-rutrackerSessions
	}
	httpreq.Header.Set("Cookie", "bb_session="+rusession+";")
	httpreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(httpreq)
	if err != nil {
		return errors.Wrap(err, "Failed request this url")
	}

	tor := &torrent.Torrent{}
	tor.TrackerTorrentID = pointer.ToString(u.Query().Get("t"))
	tor.TrackerName = pointer.ToString("rutracker")

	defer res.Body.Close()
	if res.ContentLength > 1024*1024*100 {
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_torrent = tryed_fetch_torrent+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.New("Too big response for torrent file")
	}

	if !strings.Contains(res.Header.Get("Content-Type"), "bittorrent") {
		for i := 0; i < 100; i++ {
			<-rutrackerSessions // may be bad session, use some of them so there will be only 10 failed jobs, not 1000
		}
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_torrent = tryed_fetch_torrent+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		// if strings.Contains(res.Header.Get("Content-Type"), "html") {
		// 	utfBody, err := iconv.NewReader(res.Body, "Windows-1251", "utf-8")
		// 	if err == nil {
		// 		body, _ := ioutil.ReadAll(utfBody)
		// 		// log.Println(string(body))
		// 	}
		// }
		return errors.New("failed read torrent file, wrong Content-Type " + res.Header.Get("Content-Type") + " " + u.String())
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_torrent = tryed_fetch_torrent+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.Wrap(err, "failed read torrent file body "+u.String())
	} else if len(buf) < 10 {
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_torrent = tryed_fetch_torrent+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.New("failed read torrent file body, too small" + u.String())
	}

	tor.TorrentSourceFile = &buf
	query, args := tor.Fields().NotNil().Upsert("gongo.torrent", "tracker_name", "tracker_torrent_id")
	if query == "" || args == nil {
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_torrent = tryed_fetch_torrent+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.New("fetch torrent page empty upsert torrent query")
	}
	if _, err := tx.Exec(query, args...); err != nil {
		tx.Exec("UPDATE gongo.torrent SET tryed_fetch_torrent = tryed_fetch_torrent+1 WHERE tracker_name = 'rutracker' and tracker_torrent_id = $1", *tor.TrackerTorrentID)
		return errors.Wrapf(err, "Failed upsert torrent with id=%s", tor.TrackerTorrentID)
	}
	return nil
}

func CheckRutrackerSession(session string) bool {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	httpreq, err := http.NewRequest("GET", "http://"+ENV_RUTRACKER_HOST+"/forum/tracker.php", nil)
	httpreq.Header.Set("Cookie", "bb_session="+session)
	resp, err := client.Do(httpreq)

	if err != nil {
		return false
	}
	return resp.StatusCode == 200
}

func GenNewRutrackerSession(user string) (string, error) {
	setCookie := ""
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.Response != nil {
				cookie := req.Response.Header.Get("Set-Cookie")
				if cookie != "" {
					setCookie = cookie
				}
			}
			return nil
		},
	}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	data := url.Values{}
	data.Add("login_password", user)
	data.Add("login_username", user)
	data.Add("login", "Вход")

	httpreq, err := http.NewRequest("POST", "http://"+ENV_RUTRACKER_HOST+"/forum/login.php", strings.NewReader(data.Encode()))
	httpreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return "", errors.Wrap(err, "Failed http.NewRequest first rutracker login request")
	}
	resp, err := client.Do(httpreq)
	if err != nil {
		return "", errors.Wrap(err, "Failed fetch first rutracker login request")
	}
	if setCookie == "" {
		setCookie = resp.Header.Get("Set-Cookie")
	}
	if setCookie == "" {
		log.Println("Read captcha")
		utfBody, err := iconv.NewReader(resp.Body, "Windows-1251", "utf-8")
		if err != nil {
			return "", errors.Wrap(err, "Failed create iconv.NewReader for Windows-1251")
		}
		body, err := ioutil.ReadAll(utfBody)
		if err != nil {
			return "", errors.Wrap(err, "Failed read html body rutracker login to find captcha")
		}
		csid, ccode, captcha, err := DecodeCaptchaFromHtml(string(body))
		if err != nil {
			return "", errors.Wrap(err, "Failed decode captcha from html")
		}
		data := url.Values{}
		data.Add("login_password", user)
		data.Add("login_username", user)
		data.Add("login", "Вход")
		data.Add("cap_sid", csid)
		data.Add(ccode, captcha)

		httpreq, err := http.NewRequest("POST", "http://"+ENV_RUTRACKER_HOST+"/forum/login.php", strings.NewReader(data.Encode()))
		if err != nil {
			return "", errors.Wrap(err, "Failed http.NewRequest second rutracker login request")
		}
		httpreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(httpreq)
		if err != nil {
			return "", errors.Wrap(err, "Failed fetch second rutracker login request")
		}
		if setCookie == "" {
			setCookie = resp.Header.Get("Set-Cookie")
		}
	}
	if setCookie == "" {
		return "", errors.New("rutracker login have no cookie yet")
	}

	cre := regexp.MustCompile(`(?s)bb_session=([^;]+);`)
	session := cre.FindStringSubmatch(setCookie)
	if session == nil || session[1] == "" {
		return "", errors.New("rutracker login no cookie in set cookie")
	}
	log.Println("New rutracker session", session[1])
	return session[1], nil
}

func DecodeCaptchaFromHtml(html string) (sid, code, captcha string, err error) {
	client := &http.Client{}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	var re = regexp.MustCompile(`(?s)src="([^"]+captcha[^"]+)".*name="cap_sid"\s+value="([^"]+)".*name="(cap_code_[^"]+)"`)
	found := re.FindStringSubmatch(html)
	if len(found) != 4 {
		// log.Println(html)
		return "", "", "", errors.New("rutracker Failed find captcha in response html")
	}
	captchahref, err := urlx.Parse(found[1])
	if err != nil {
		return "", "", "", errors.New("rutracker login Failed parse captcha href")
	}
	trackerhref, _ := urlx.Parse("http://" + ENV_RUTRACKER_HOST + "/forum/login.php")
	captchahref = trackerhref.ResolveReference(captchahref)
	captchares, err := client.Get(captchahref.String())
	if err != nil {
		return "", "", "", errors.New("rutracker login Failed get captcha image")
	}
	var captchareqbuf bytes.Buffer
	w := multipart.NewWriter(&captchareqbuf)
	fcaptcha, err := w.CreateFormFile("captcha", "captcha.jpg")
	if err != nil {
		return "", "", "", errors.New("CreateFormField captcha")
	}
	if b, err := io.Copy(fcaptcha, captchares.Body); err != nil || b < 10 {
		return "", "", "", errors.New("rutracker login Failed copy captcha to field")
	}
	fkey, _ := w.CreateFormField("key")
	fkey.Write([]byte(ENV_CAPTCHA_DECODE_KEY))
	fmethod, _ := w.CreateFormField("method")
	fmethod.Write([]byte("solve"))
	w.Close()
	req, err := http.NewRequest("POST", ENV_CAPTCHA_DECODE_URL, &captchareqbuf)
	if err != nil {
		return "", "", "", errors.Wrap(err, "Failed create request captcha decode")
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	log.Println("Decode captcha")
	rescaptchadecoded, err := client.Do(req)
	if err != nil {
		return "", "", "", errors.Wrap(err, "Failed request captcha decode")
	}
	decodedbuf, err := ioutil.ReadAll(rescaptchadecoded.Body)
	if err != nil {
		return "", "", "", errors.Wrap(err, "Failed read captcha decodedbuf")
	}
	decoded := strings.Split(string(decodedbuf), "|")
	if len(decoded) < 3 || len(decoded[2]) < 1 {
		return "", "", "", errors.New("Failed read captcha decodedbuf " + string(decodedbuf))
	}
	log.Println("Decode captcha OK", decoded[2], captchahref.String())
	return found[2], found[3], decoded[2], nil
}

var GetRutrackerSessionMutex = &sync.Mutex{}

func GetRutrackerSession(user string) (session string, err error) {
	for i := 0; ; i++ {
		session, err = GenNewRutrackerSession(user)
		if err == nil || i > 4 {
			break
		}
		log.Printf("%+v\n", errors.Wrap(err, "Failed gen rutracker session"))
	}
	return session, err
}

func RegisterNewRutrackerUser() (user string, err error) {
	t := time.Now()
	user = "go" + fmt.Sprintf("%d%02d%02d%02d%02d%02d%03d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000000)
	log.Println("Register user " + user)
	client := &http.Client{}
	if ProxyURL != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(ProxyURL)}
	}
	data := url.Values{}
	data.Add("form_token", "")
	data.Add("reg_agreed", "1")

	httpreq, err := http.NewRequest("POST", "http://"+ENV_RUTRACKER_HOST+"/forum/profile.php?mode=register", strings.NewReader(data.Encode()))
	httpreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return "", errors.Wrap(err, "Failed http.NewRequest first profile.php?mode=register request")
	}
	resp, err := client.Do(httpreq)
	if err != nil {
		return "", errors.Wrap(err, "Failed fetch first profile.php?mode=register request")
	}
	utfBody, err := iconv.NewReader(resp.Body, "Windows-1251", "utf-8")
	if err != nil {
		return "", errors.Wrap(err, "Failed create iconv.NewReader for Windows-1251")
	}

	body, err := ioutil.ReadAll(utfBody)
	if err != nil {
		return "", errors.Wrap(err, "Failed read html body rutracker profile.php?mode=register to find captcha")
	}
	csid, ccode, captcha, err := DecodeCaptchaFromHtml(string(body))
	if err != nil {
		return "", errors.Wrap(err, "Failed decode captcha from html")
	}
	data = url.Values{}
	data.Add("reg_agreed", "1")
	data.Add("username", user)
	data.Add("new_pass", user)
	data.Add("user_email", user+"@nirhub.ru")
	data.Add("user_flag_id", "0")
	data.Add("user_timezone_x2", "6")
	data.Add("user_gender_id", "0")
	data.Add("submit", "Email указан верно")
	data.Add("cap_sid", csid)
	data.Add(ccode, captcha)

	httpreq, err = http.NewRequest("POST", "http://"+ENV_RUTRACKER_HOST+"/forum/profile.php?mode=register", strings.NewReader(data.Encode()))
	if err != nil {
		return "", errors.Wrap(err, "Failed http.NewRequest second profile.php?mode=register request")
	}
	httpreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = client.Do(httpreq)
	if err != nil {
		return "", errors.Wrap(err, "Failed fetch second profile.php?mode=register request")
	}
	if resp.StatusCode != 200 {
		return "", errors.New("Failed fetch second profile.php?mode=register request, status code not 200: " + resp.Status)
	}
	utfBody, err = iconv.NewReader(resp.Body, "Windows-1251", "utf-8")
	if err != nil {
		return "", errors.Wrap(err, "Failed create iconv.NewReader for Windows-1251")
	}

	body, err = ioutil.ReadAll(utfBody)
	if err != nil {
		return "", errors.Wrap(err, "Failed read html body rutracker profile.php?mode=register")
	}
	if !strings.Contains(string(body), "Следуйте указанным в нём инструкциям") {
		log.Println(string(body))
		return "", errors.Wrap(err, "Failed read html body rutracker profile.php?mode=register")
	}
	mail, err := WaitForMail(user)
	if err != nil {
		return "", errors.Wrap(err, "Failed wait for mail for "+user)
	}
	link := regexp.MustCompile(`(?s)\s(\S+profile.php\?mode=activate\S+)\s`).FindStringSubmatch(mail)
	if len(link) < 2 {
		return "", errors.New("Failed find activation link in mail " + mail)
	}
	resp, err = client.Get(link[1])
	if err != nil {
		return "", errors.Wrap(err, "Failed fetch activation link "+link[1])
	}

	return user, nil
}

var rutrackerSessions = make(chan string, 500) // 1000 max torrents for user
func StartRutrackerSessionsGenerator() {
	log.Println("Wait for someone need rutracker session to gen it")
	for i := 0; i < cap(rutrackerSessions)+1; i++ {
		rutrackerSessions <- "" // dont gen sessions and users until no one need
	}
	for {
		user, err := RegisterNewRutrackerUser()
		if err != nil {
			log.Printf("%+v\n", errors.Wrap(err, "Failed create new rutracker user"))
			time.Sleep(time.Second * 30)
			continue
		}
		session, err := GetRutrackerSession(user)
		if err != nil {
			log.Printf("%+v\n", errors.Wrap(err, "Failed create new rutracker session for user "+user))
			time.Sleep(time.Second * 5)
			continue
		}
		rutrackerSession.Store(session)
		for i := 0; i < 999; i++ {
			rutrackerSessions <- session
		}
	}
}

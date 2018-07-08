package parser

import (
	"time"

	"github.com/avezila/gongo/esql"
)

//Proxy represents row in table_proxy
type Proxy struct {
	tableName struct{} `sql:"gongo.proxy"`

	IP *string

	Delay          *float32
	QPS            *float32
	TotalRequests  *int64
	FailedRequests *int64

	HTTP   *bool
	HTTPS  *bool
	Socks4 *bool
	Socks5 *bool

	Broken          *bool
	DomainRuTracker *bool `sql:"domain_rutracker"`
	DomainNNMClub   *bool `sql:"domain_nnmclub"`

	DBAddTime   *time.Time
	DBCheckTime *time.Time
	Modified    *time.Time
}

func (p *Proxy) Fields() esql.Fields {
	return esql.Fields{
		{"IP", "ip", &p.IP, p.IP == nil || *p.IP == ""},
		{"Delay", "delay", &p.Delay, p.Delay == nil || *p.Delay == 0},
		{"QPS", "qps", &p.QPS, p.QPS == nil || *p.QPS == 0},
		{"TotalRequests", "total_requests", &p.TotalRequests, p.TotalRequests == nil || *p.TotalRequests == 0},
		{"FailedRequests", "failed_requests", &p.FailedRequests, p.FailedRequests == nil || *p.FailedRequests == 0},
		{"HTTP", "http", &p.HTTP, p.HTTP == nil || *p.HTTP == false},
		{"HTTPS", "https", &p.HTTPS, p.HTTPS == nil || *p.HTTPS == false},
		{"Socks4", "socks4", &p.Socks4, p.Socks4 == nil || *p.Socks4 == false},
		{"Socks5", "socks5", &p.Socks5, p.Socks5 == nil || *p.Socks5 == false},
		{"Broken", "broken", &p.Broken, p.Broken == nil || *p.Broken == false},
		{"DomainRuTracker", "domain_rutracker", &p.DomainRuTracker, p.DomainRuTracker == nil || *p.DomainRuTracker == false},
		{"DomainNNMClub", "domain_nnmclub", &p.DomainNNMClub, p.DomainNNMClub == nil || *p.DomainNNMClub == false},
		{"DBAddTime", "db_add_time", &p.DBAddTime, p.DBAddTime == nil || p.DBAddTime.IsZero()},
		{"DBCheckTime", "db_check_time", &p.DBCheckTime, p.DBCheckTime == nil || p.DBCheckTime.IsZero()},
		{"Modified", "modified", &p.Modified, p.Modified == nil || p.Modified.IsZero()},
	}
}

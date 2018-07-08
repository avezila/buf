package parser

//TrackerName represents string as name of tracker
type TrackerName string

const (
	//RuTracker name of most popular torrent tracker
	RuTracker TrackerName = "rutracker"
	//NNMClub name of second popular torrent tracker
	NNMClub TrackerName = "nnmclub"
)

//Tracker struct for useful parametres of trackers(it shouldnt be in database)
type Tracker struct {
	Domain string
	Name   TrackerName
}

//Trackers map of common trackers
var Trackers = map[TrackerName]Tracker{
	RuTracker: Tracker{
		Domain: "http://" + ENV_RUTRACKER_HOST + "/",
		Name:   RuTracker,
	},
	NNMClub: Tracker{
		Domain: "http://nnm-club.name/",
		Name:   NNMClub,
	},
}

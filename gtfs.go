package gtfs

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"

	tablib "github.com/agrison/go-tablib"
)

type Feed struct {
	Dir             string
	Routes          map[string]*Route
	Shapes          map[string]*Shape
	Stops           map[string]*Stop
	Trips           map[string]*Trip
	CalendarEntries map[string]CalendarEntry
}

type Route struct {
	Id        string
	ShortName string
	LongName  string
	Trips     []*Trip
}

type Trip struct {
	Id        string
	Shape     *Shape
	Route     *Route
	Service   string
	Direction string
	Headsign  string

	// may not be loaded
	StopTimes []StopTime
}

type Headsign struct {
	Direction string
	Text      string
}

type Shape struct {
	Id     string
	Coords []Coord
}

type Stop struct {
	Id    string
	Name  string
	Coord Coord
}

type StopTime struct {
	Stop *Stop
	Trip *Trip
	Time int
	Seq  int
}

type CalendarEntry struct {
	ServiceId string
	Days      []string
}

type StopTimeBySeq []StopTime

func (a StopTimeBySeq) Len() int           { return len(a) }
func (a StopTimeBySeq) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a StopTimeBySeq) Less(i, j int) bool { return a[i].Seq < a[j].Seq }

type Coord struct {
	Lat float64
	Lon float64
	Seq int
}

type CoordBySeq []Coord

func (a CoordBySeq) Len() int           { return len(a) }
func (a CoordBySeq) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a CoordBySeq) Less(i, j int) bool { return a[i].Seq < a[j].Seq }

// main utility function for reading GTFS files
func (feed *Feed) readCsv(filename string, f func(map[string]interface{})) error {
	fileData, err := ioutil.ReadFile(path.Join(feed.Dir, filename))
	if err != nil {
		return err
	}
	dataset, err := tablib.LoadCSV(fileData)
	if err != nil {
		return err
	}

	// need to build list of rows to grab
	rowIDs := make([]int, dataset.Height())
	for i := range rowIDs {
		rowIDs[i] = i
	}

	fmt.Println(filename, dataset.Height())
	rows, err := dataset.Rows(rowIDs...)
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not load rows: %s", err.Error()))
	}
	for _, row := range rows {
		f(row)
	}

	return nil
}

func Load(feed_path string, loadStopTimes bool) Feed {
	f := Feed{Dir: feed_path}
	f.Routes = make(map[string]*Route)
	f.Shapes = make(map[string]*Shape)
	f.Stops = make(map[string]*Stop)
	f.Trips = make(map[string]*Trip)
	f.CalendarEntries = make(map[string]CalendarEntry)

	f.readCsv("calendar.txt", func(s map[string]interface{}) {
		c := CalendarEntry{ServiceId: s["service_id"].(string), Days: []string{s["monday"].(string), s["tuesday"].(string), s["wednesday"].(string), s["thursday"].(string), s["friday"].(string), s["saturday"].(string), s["sunday"].(string)}}
		f.CalendarEntries[s["service_id"].(string)] = c
	})

	// we assume that this CSV is grouped by shape_id
	// but this is not guaranteed in spec?
	var curShape *Shape
	var found = false
	f.readCsv("shapes.txt", func(s map[string]interface{}) {
		shape_id := s["shape_id"].(string)
		if !found || shape_id != curShape.Id {
			if found {
				f.Shapes[curShape.Id] = curShape
			}
			found = true
			curShape = &Shape{Id: shape_id}
		}
		lon, _ := strconv.ParseFloat(s["shape_pt_lon"].(string), 64)
		lat, _ := strconv.ParseFloat(s["shape_pt_lat"].(string), 64)
		seq, _ := strconv.Atoi(s["shape_pt_sequence"].(string))
		curShape.Coords = append(curShape.Coords, Coord{Lat: lat, Lon: lon, Seq: seq})
	})
	if found {
		f.Shapes[curShape.Id] = curShape
	}

	// sort coords by their sequence
	for _, v := range f.Shapes {
		sort.Sort(CoordBySeq(v.Coords))
	}

	f.readCsv("routes.txt", func(s map[string]interface{}) {
		rsn := strings.TrimSpace(s["route_short_name"].(string))
		rln := strings.TrimSpace(s["route_long_name"].(string))
		id := strings.TrimSpace(s["route_id"].(string))
		f.Routes[id] = &Route{Id: id, ShortName: rsn, LongName: rln}
	})

	f.readCsv("trips.txt", func(s map[string]interface{}) {
		route_id := s["route_id"].(string)
		service := s["service_id"].(string)
		trip_id := s["trip_id"].(string)
		direction := s["direction_id"].(string)
		shape_id := s["shape_id"].(string)
		headsign := s["trip_headsign"].(string)

		var shape *Shape
		shape = f.Shapes[shape_id]
		var trip Trip
		trip.StopTimes = []StopTime{}
		f.Trips[trip_id] = &trip

		route := f.Routes[route_id]
		trip = Trip{Shape: shape, Route: route, Id: trip_id, Direction: direction, Service: service, Headsign: headsign}
		route.Trips = append(route.Trips, &trip)
		f.Routes[route_id] = route
	})

	f.readCsv("stops.txt", func(s map[string]interface{}) {
		stop_id := s["stop_id"].(string)
		stop_name := s["stop_name"].(string)
		stop_lat, _ := strconv.ParseFloat(s["stop_lat"].(string), 64)
		stop_lon, _ := strconv.ParseFloat(s["stop_lon"].(string), 64)
		coord := Coord{Lat: stop_lat, Lon: stop_lon}
		f.Stops[stop_id] = &Stop{Coord: coord, Name: stop_name, Id: stop_id}
	})

	if !loadStopTimes {
		return f
	}
	f.readCsv("stop_times.txt", func(s map[string]interface{}) {
		trip_id := s["trip_id"].(string)
		stop_id := s["stop_id"].(string)
		seq, _ := strconv.Atoi(s["stop_sequence"].(string))
		time := Hmstoi(s["arrival_time"].(string))
		stop := f.Stops[stop_id]
		trip := f.Trips[trip_id]
		newStopTime := StopTime{Trip: trip, Stop: stop, Seq: seq, Time: time}
		trip.StopTimes = append(trip.StopTimes, newStopTime)
	})

	// sort stops by seq

	for _, v := range f.Trips {
		sort.Sort(StopTimeBySeq(v.StopTimes))
	}

	return f
}

func (feed *Feed) RouteByShortName(shortName string) *Route {
	for _, v := range feed.Routes {
		if v.ShortName == shortName {
			return v
		}
	}
	//TODO error here
	return &Route{}
}

// get All shapes for a route
func (route Route) Shapes() []*Shape {
	// collect the unique list of shape pointers
	hsh := make(map[*Shape]bool)

	for _, v := range route.Trips {
		hsh[v.Shape] = true
	}

	retval := []*Shape{}
	for k, _ := range hsh {
		retval = append(retval, k)
	}
	return retval
}

func (route Route) LongestShape() *Shape {
	max := 0
	var shape *Shape
	for _, s := range route.Shapes() {
		if len(s.Coords) > max {
			shape = s
			max = len(s.Coords)
		}
	}
	return shape
}

func Hmstoi(str string) int {
	components := strings.Split(str, ":")
	hour, _ := strconv.Atoi(components[0])
	min, _ := strconv.Atoi(components[1])
	sec, _ := strconv.Atoi(components[2])
	retval := hour*60*60 + min*60 + sec
	return retval
}

func (route Route) Stops() []*Stop {
	stops := make(map[*Stop]bool)
	// can't assume the longest shape includes all stops

	for _, t := range route.Trips {
		for _, st := range t.StopTimes {
			stops[st.Stop] = true
		}
	}

	retval := []*Stop{}
	for k, _ := range stops {
		retval = append(retval, k)
	}
	return retval
}

func (route Route) Headsigns() []string {
	max0 := 0
	maxHeadsign0 := ""
	max1 := 1
	maxHeadsign1 := ""

	for _, t := range route.Trips {
		if t.Direction == "0" {
			if len(t.Shape.Coords) > max0 {
				max0 = len(t.Shape.Coords)
				maxHeadsign0 = strings.TrimSpace(t.Headsign)
			}
		} else { // direction == 1. only bidirectional
			if len(t.Shape.Coords) > max1 {
				max1 = len(t.Shape.Coords)
				maxHeadsign1 = strings.TrimSpace(t.Headsign)
			}
		}
	}

	return []string{maxHeadsign0, maxHeadsign1}
}

func (feed Feed) Calendar() []string {
	retval := []string{}
	for i := 0; i <= 6; i++ {
		for k, v := range feed.CalendarEntries {
			if v.Days[i] == "1" {
				retval = append(retval, k)
			}
		}
	}
	return retval
}

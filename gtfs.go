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

// Feed represents a collection of GTFS information.
type Feed struct {
	Dir             string
	Routes          map[string]*Route
	Shapes          map[string]*Shape
	Stops           map[string]*Stop
	Trips           map[string]*Trip
	CalendarEntries map[string]CalendarEntry
}

// RouteType describes the type of vehicle uses a particular route.
type RouteType int

const (
	LightRail RouteType = 0
	Subway    RouteType = 1
	Rail      RouteType = 2
	Bus       RouteType = 3
	Ferry     RouteType = 4
	CableCar  RouteType = 5
	Gondola   RouteType = 6
	Funicular RouteType = 7
)

// Route represents a single "line", and is made up of one or more trips.
type Route struct {
	ID          string
	AgencyID    *string
	ShortName   string
	LongName    string
	Description *string
	Type        RouteType
	URL         *string
	Color       *string
	TextColor   *string
	Trips       []*Trip
}

// Trip reprents a journey taken by a vehicle through stops.
type Trip struct {
	ID        string
	Shape     *Shape
	Route     *Route
	Service   string
	Direction string
	Headsign  string

	// may not be loaded
	StopTimes []StopTime
}

// Headsign is the text representing a trip that appears on a sign, such as a bus or train display.
type Headsign struct {
	Direction string
	Text      string
}

// Shape describes the physical path that a vehicle takes along a Trip.
type Shape struct {
	ID     string
	Coords []Coord
}

// Stop represents a location where vehicles stop to pick up or drop off passengers.
type Stop struct {
	ID    string
	Name  string
	Coord Coord
}

// StopTime defines when a vehicle arrives at a location, how long it stays there, and when it departs.
type StopTime struct {
	Stop *Stop
	Trip *Trip
	Time int
	Seq  int
}

type CalendarEntry struct {
	ServiceID string
	Days      []string
}

// StopTimeBySeq is used to sort StopTimes by their sequence number.
type StopTimeBySeq []StopTime

func (a StopTimeBySeq) Len() int           { return len(a) }
func (a StopTimeBySeq) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a StopTimeBySeq) Less(i, j int) bool { return a[i].Seq < a[j].Seq }

// Coord represents a coordinate.
type Coord struct {
	Lat float64
	Lon float64
	Seq int
}

// CoordBySeq is used to sort Coords by their sequence number.
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

// Load retrieves data from the given directory path and returns a Feed containing that data.
func Load(feedPath string, loadStopTimes bool) Feed {
	f := Feed{Dir: feedPath}
	f.Routes = make(map[string]*Route)
	f.Shapes = make(map[string]*Shape)
	f.Stops = make(map[string]*Stop)
	f.Trips = make(map[string]*Trip)
	f.CalendarEntries = make(map[string]CalendarEntry)

	f.readCsv("calendar.txt", func(s map[string]interface{}) {
		c := CalendarEntry{ServiceID: s["service_id"].(string), Days: []string{s["monday"].(string), s["tuesday"].(string), s["wednesday"].(string), s["thursday"].(string), s["friday"].(string), s["saturday"].(string), s["sunday"].(string)}}
		f.CalendarEntries[s["service_id"].(string)] = c
	})

	// we assume that this CSV is grouped by shape_id
	// but this is not guaranteed in spec?
	var curShape *Shape
	var found = false
	f.readCsv("shapes.txt", func(s map[string]interface{}) {
		shapeID := s["shape_id"].(string)
		if !found || shapeID != curShape.ID {
			if found {
				f.Shapes[curShape.ID] = curShape
			}
			found = true
			curShape = &Shape{ID: shapeID}
		}
		lon, _ := strconv.ParseFloat(s["shape_pt_lon"].(string), 64)
		lat, _ := strconv.ParseFloat(s["shape_pt_lat"].(string), 64)
		seq, _ := strconv.Atoi(s["shape_pt_sequence"].(string))
		curShape.Coords = append(curShape.Coords, Coord{Lat: lat, Lon: lon, Seq: seq})
	})
	if found {
		f.Shapes[curShape.ID] = curShape
	}

	// sort coords by their sequence
	for _, v := range f.Shapes {
		sort.Sort(CoordBySeq(v.Coords))
	}

	f.readCsv("routes.txt", func(s map[string]interface{}) {
		rsn := strings.TrimSpace(s["route_short_name"].(string))
		rln := strings.TrimSpace(s["route_long_name"].(string))
		id := strings.TrimSpace(s["route_id"].(string))
		var aid *string
		if s["agency_id"] != nil {
			aidString := strings.TrimSpace(s["agency_id"].(string))
			aid = &aidString
		}
		var desc *string
		if s["description"] != nil {
			descString := strings.TrimSpace(s["description"].(string))
			desc = &descString
		}
		var url *string
		if s["url"] != nil {
			urlString := strings.TrimSpace(s["url"].(string))
			url = &urlString
		}
		var color *string
		if s["route_color"] != nil {
			colorString := strings.TrimSpace(s["route_color"].(string))
			color = &colorString
		}
		var textColor *string
		if s["text_color"] != nil {
			textColorString := strings.TrimSpace(s["text_color"].(string))
			textColor = &textColorString
		}
		// we assume this will always be right
		routeTypeInt, _ := strconv.Atoi(s["route_type"].(string))
		routeTypeID := RouteType(routeTypeInt)
		f.Routes[id] = &Route{
			ID:          id,
			AgencyID:    aid,
			ShortName:   rsn,
			LongName:    rln,
			Description: desc,
			Type:        routeTypeID,
			URL:         url,
			Color:       color,
			TextColor:   textColor,
		}
	})

	f.readCsv("trips.txt", func(s map[string]interface{}) {
		routeID := s["route_id"].(string)
		service := s["service_id"].(string)
		tripID := s["trip_id"].(string)
		direction := s["direction_id"].(string)
		shapeID := s["shape_id"].(string)
		headsign := s["trip_headsign"].(string)

		var shape *Shape
		shape = f.Shapes[shapeID]
		var trip Trip
		trip.StopTimes = []StopTime{}
		f.Trips[tripID] = &trip

		route := f.Routes[routeID]
		trip = Trip{Shape: shape, Route: route, ID: tripID, Direction: direction, Service: service, Headsign: headsign}
		route.Trips = append(route.Trips, &trip)
		f.Routes[routeID] = route
	})

	f.readCsv("stops.txt", func(s map[string]interface{}) {
		stopID := s["stop_id"].(string)
		stopName := s["stop_name"].(string)
		stopLat, _ := strconv.ParseFloat(s["stop_lat"].(string), 64)
		stopLon, _ := strconv.ParseFloat(s["stop_lon"].(string), 64)
		coord := Coord{Lat: stopLat, Lon: stopLon}
		f.Stops[stopID] = &Stop{Coord: coord, Name: stopName, ID: stopID}
	})

	if !loadStopTimes {
		return f
	}
	f.readCsv("stop_times.txt", func(s map[string]interface{}) {
		tripID := s["trip_id"].(string)
		stopID := s["stop_id"].(string)
		seq, _ := strconv.Atoi(s["stop_sequence"].(string))
		time := hmstoi(s["arrival_time"].(string))
		stop := f.Stops[stopID]
		trip := f.Trips[tripID]
		newStopTime := StopTime{Trip: trip, Stop: stop, Seq: seq, Time: time}
		trip.StopTimes = append(trip.StopTimes, newStopTime)
	})

	// sort stops by seq

	for _, v := range f.Trips {
		sort.Sort(StopTimeBySeq(v.StopTimes))
	}

	return f
}

// RouteByShortName searches for and returns a route based on its short name, if it exists.
func (feed *Feed) RouteByShortName(shortName string) *Route {
	for _, v := range feed.Routes {
		if v.ShortName == shortName {
			return v
		}
	}
	return nil
}

// Shapes returns all shapes for a route.
func (route Route) Shapes() []*Shape {
	// collect the unique list of shape pointers
	hsh := make(map[*Shape]bool)

	for _, v := range route.Trips {
		hsh[v.Shape] = true
	}

	retval := []*Shape{}
	for k := range hsh {
		retval = append(retval, k)
	}
	return retval
}

// LongestShape returns the longest shape in the route.
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

// hmstoi returns the number of seconds for a given time string.
func hmstoi(str string) int {
	components := strings.Split(str, ":")
	hour, _ := strconv.Atoi(components[0])
	min, _ := strconv.Atoi(components[1])
	sec, _ := strconv.Atoi(components[2])
	retval := hour*60*60 + min*60 + sec
	return retval
}

// Stops returns all the stops on this route.
func (route Route) Stops() []*Stop {
	stops := make(map[*Stop]bool)
	// can't assume the longest shape includes all stops

	for _, t := range route.Trips {
		for _, st := range t.StopTimes {
			stops[st.Stop] = true
		}
	}

	retval := []*Stop{}
	for k := range stops {
		retval = append(retval, k)
	}
	return retval
}

// Headsigns returns the two headsigns for this route.
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

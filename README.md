# Go.GTFS

Simple Go library for loading and using static GTFS data.

---

[![GoDoc](https://godoc.org/github.com/DanielOaks/go.gtfs?status.svg)](https://godoc.org/github.com/DanielOaks/go.gtfs)
[![Go Report Card](https://goreportcard.com/badge/github.com/DanielOaks/go.gtfs)](https://goreportcard.com/report/github.com/DanielOaks/go.gtfs)

---

Install with:

	go get "github.com/DanielOaks/go.gtfs"

Use with:

	import "github.com/DanielOaks/go.gtfs"

## Examples

Examples assume you have directory called `sf_muni` containing GTFS files.

	feed := gtfs.Load("sf_muni")
	route := feed.RouteByShortName("N")
	coords := route.Shapes()[0].Coords

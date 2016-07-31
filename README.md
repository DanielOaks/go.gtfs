
	import "github.com/DanielOaks/go.gtfs"

## Examples

Examples assume you have directory called `sf_muni` containing GTFS files.

	feed := gtfs.Load("sf_muni")
	route := feed.RouteByShortName("N")
	coords := route.Shapes()[0].Coords

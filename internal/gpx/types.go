package gpx

type Point struct {
	Lat  float64
	Lon  float64
	Ele  *float64
	Time string
}

type Segment struct {
	Name   string
	Points []Point
}

type Track struct {
	Name     string
	Segments []Segment
}

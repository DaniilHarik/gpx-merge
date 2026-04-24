package gpx

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type xmlGPX struct {
	XMLName xml.Name   `xml:"gpx"`
	Tracks  []xmlTrack `xml:"trk"`
}

type xmlTrack struct {
	Name     string       `xml:"name"`
	Segments []xmlSegment `xml:"trkseg"`
}

type xmlSegment struct {
	Name   string     `xml:"name"`
	Points []xmlPoint `xml:"trkpt"`
}

type xmlPoint struct {
	Lat  float64  `xml:"lat,attr"`
	Lon  float64  `xml:"lon,attr"`
	Ele  *float64 `xml:"ele"`
	Time string   `xml:"time"`
}

func ParseFile(ctx context.Context, path string, relPath string) ([]Track, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := xml.NewDecoder(&contextReader{ctx: ctx, r: f})
	var in xmlGPX
	if err := dec.Decode(&in); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("decode xml: %w", err)
	}
	if len(in.Tracks) == 0 {
		return nil, fmt.Errorf("no <trk> elements found")
	}

	base := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	unnamedN := 0
	unnamedTotal := 0
	for _, trk := range in.Tracks {
		if strings.TrimSpace(trk.Name) == "" {
			unnamedTotal++
		}
	}

	out := make([]Track, 0, len(in.Tracks))
	for _, trk := range in.Tracks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(trk.Name)
		if name == "" {
			if unnamedTotal <= 1 {
				name = base
			} else {
				unnamedN++
				name = fmt.Sprintf("%s #%d", base, unnamedN)
			}
		}

		segments := make([]Segment, 0, len(trk.Segments))
		for _, seg := range trk.Segments {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			pts := make([]Point, 0, len(seg.Points))
			for _, p := range seg.Points {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if p.Lat < -90 || p.Lat > 90 {
					return nil, fmt.Errorf("invalid latitude %.6f: must be in [-90, 90]", p.Lat)
				}
				if p.Lon < -180 || p.Lon > 180 {
					return nil, fmt.Errorf("invalid longitude %.6f: must be in [-180, 180]", p.Lon)
				}
				pts = append(pts, Point{
					Lat:  p.Lat,
					Lon:  p.Lon,
					Ele:  p.Ele,
					Time: strings.TrimSpace(p.Time),
				})
			}
			segments = append(segments, Segment{
				Name:   strings.TrimSpace(seg.Name),
				Points: pts,
			})
		}

		out = append(out, Track{Name: name, Segments: segments})
	}

	return out, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

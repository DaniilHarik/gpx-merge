package gpx

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"gpx-merge/internal/geo"
	"gpx-merge/internal/optimize"
)

type WriteOptions struct {
	Creator             string
	Precision           int
	KeepTime            bool
	KeepEle             bool
	SplitTrackGapMeters float64
	IncludeMetadata     bool
	MetadataDesc        string
	MetadataTime        time.Time
}

func WriteMerged(w io.Writer, tracks []Track, opts WriteOptions) (int64, error) {
	cw := &countingWriter{w: w}
	if _, err := io.WriteString(cw, xml.Header); err != nil {
		return cw.n, err
	}

	enc := xml.NewEncoder(cw)
	root := xml.StartElement{
		Name: xml.Name{Local: "gpx"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: "http://www.topografix.com/GPX/1/1"},
			{Name: xml.Name{Local: "version"}, Value: "1.1"},
			{Name: xml.Name{Local: "creator"}, Value: opts.Creator},
		},
	}
	if err := enc.EncodeToken(root); err != nil {
		return cw.n, err
	}

	if opts.IncludeMetadata {
		if err := writeMetadata(enc, opts); err != nil {
			return cw.n, err
		}
	}
	if err := writeTracks(enc, tracks, opts); err != nil {
		return cw.n, err
	}
	if err := enc.EncodeToken(root.End()); err != nil {
		return cw.n, err
	}
	if err := enc.Flush(); err != nil {
		return cw.n, err
	}
	return cw.n, nil
}

func MeasureMerged(tracks []Track, opts WriteOptions) (int64, error) {
	return WriteMerged(io.Discard, tracks, opts)
}

func MeasureTracks(tracks []Track, opts WriteOptions) (int64, error) {
	cw := &countingWriter{w: io.Discard}
	enc := xml.NewEncoder(cw)
	if err := writeTracks(enc, tracks, opts); err != nil {
		return cw.n, err
	}
	if err := enc.Flush(); err != nil {
		return cw.n, err
	}
	return cw.n, nil
}

func writeMetadata(enc *xml.Encoder, opts WriteOptions) error {
	metadata := xml.StartElement{Name: xml.Name{Local: "metadata"}}
	if err := enc.EncodeToken(metadata); err != nil {
		return err
	}

	ts := opts.MetadataTime
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	if err := writeTextElement(enc, "time", ts.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if opts.MetadataDesc != "" {
		if err := writeTextElement(enc, "desc", opts.MetadataDesc); err != nil {
			return err
		}
	}

	return enc.EncodeToken(metadata.End())
}

func writeTracks(enc *xml.Encoder, tracks []Track, opts WriteOptions) error {
	for _, trk := range tracks {
		validSegments := make([]Segment, 0, len(trk.Segments))
		for _, seg := range trk.Segments {
			if len(seg.Points) >= 2 {
				validSegments = append(validSegments, seg)
			}
		}
		if len(validSegments) == 0 {
			continue
		}

		segmentGroups := splitSegmentsByGap(validSegments, opts.SplitTrackGapMeters)
		for i, group := range segmentGroups {
			trkStart := xml.StartElement{Name: xml.Name{Local: "trk"}}
			if err := enc.EncodeToken(trkStart); err != nil {
				return err
			}
			name := trackNamePart(trk.Name, i, len(segmentGroups))
			if name != "" {
				if err := writeTextElement(enc, "name", name); err != nil {
					return err
				}
			}

			for _, seg := range group {
				segStart := xml.StartElement{Name: xml.Name{Local: "trkseg"}}
				if err := enc.EncodeToken(segStart); err != nil {
					return err
				}
				if seg.Name != "" {
					if err := writeTextElement(enc, "name", seg.Name); err != nil {
						return err
					}
				}

				for _, p := range seg.Points {
					ptStart := xml.StartElement{
						Name: xml.Name{Local: "trkpt"},
						Attr: []xml.Attr{
							{Name: xml.Name{Local: "lat"}, Value: optimize.FormatCoordinate(p.Lat, opts.Precision)},
							{Name: xml.Name{Local: "lon"}, Value: optimize.FormatCoordinate(p.Lon, opts.Precision)},
						},
					}
					if err := enc.EncodeToken(ptStart); err != nil {
						return err
					}
					if opts.KeepEle && p.Ele != nil {
						if err := writeTextElement(enc, "ele", strconv.FormatFloat(*p.Ele, 'f', -1, 64)); err != nil {
							return err
						}
					}
					if opts.KeepTime && p.Time != "" {
						if err := writeTextElement(enc, "time", p.Time); err != nil {
							return err
						}
					}
					if err := enc.EncodeToken(ptStart.End()); err != nil {
						return err
					}
				}

				if err := enc.EncodeToken(segStart.End()); err != nil {
					return err
				}
			}

			if err := enc.EncodeToken(trkStart.End()); err != nil {
				return err
			}
		}
	}
	return nil
}

func splitSegmentsByGap(segments []Segment, maxGapMeters float64) [][]Segment {
	if len(segments) == 0 {
		return nil
	}
	if maxGapMeters <= 0 {
		return [][]Segment{segments}
	}

	out := make([][]Segment, 0, 1)
	current := make([]Segment, 0, len(segments))
	current = append(current, segments[0])

	for i := 1; i < len(segments); i++ {
		prev := segments[i-1]
		next := segments[i]
		if segmentEndpointGapMeters(prev, next) > maxGapMeters {
			out = append(out, current)
			current = make([]Segment, 0, len(segments)-i)
		}
		current = append(current, next)
	}

	out = append(out, current)
	return out
}

func segmentEndpointGapMeters(a, b Segment) float64 {
	endA := a.Points[len(a.Points)-1]
	startB := b.Points[0]
	return geo.HaversineMeters(endA.Lat, endA.Lon, startB.Lat, startB.Lon)
}

func trackNamePart(base string, idx int, total int) string {
	if total <= 1 || base == "" {
		return base
	}
	return fmt.Sprintf("%s (part %d)", base, idx+1)
}

func writeTextElement(enc *xml.Encoder, name, value string) error {
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData([]byte(value))); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

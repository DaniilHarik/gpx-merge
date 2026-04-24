package gpx

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
	tracks, err := parseGPX(ctx, dec)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("decode xml: %w", err)
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no <trk> elements found")
	}

	base := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	unnamedN := 0
	unnamedTotal := 0
	for _, trk := range tracks {
		if strings.TrimSpace(trk.Name) == "" {
			unnamedTotal++
		}
	}

	for i := range tracks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(tracks[i].Name)
		if name == "" {
			if unnamedTotal <= 1 {
				name = base
			} else {
				unnamedN++
				name = fmt.Sprintf("%s #%d", base, unnamedN)
			}
		}
		tracks[i].Name = name
	}

	return tracks, nil
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

func parseGPX(ctx context.Context, dec *xml.Decoder) ([]Track, error) {
	var tracks []Track
	for {
		tok, err := nextToken(ctx, dec)
		if err == io.EOF {
			return tracks, nil
		}
		if err != nil {
			return nil, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "gpx" {
			if err := skipElement(ctx, dec, start); err != nil {
				return nil, err
			}
			continue
		}
		parsed, err := parseGPXElement(ctx, dec, start)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, parsed...)
	}
}

func parseGPXElement(ctx context.Context, dec *xml.Decoder, start xml.StartElement) ([]Track, error) {
	var tracks []Track
	for {
		tok, err := nextToken(ctx, dec)
		if err != nil {
			return nil, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local != "trk" {
				if err := skipElement(ctx, dec, tok); err != nil {
					return nil, err
				}
				continue
			}
			trk, err := parseTrack(ctx, dec, tok)
			if err != nil {
				return nil, err
			}
			tracks = append(tracks, trk)
		case xml.EndElement:
			if sameElement(tok, start) {
				return tracks, nil
			}
		}
	}
}

func parseTrack(ctx context.Context, dec *xml.Decoder, start xml.StartElement) (Track, error) {
	var trk Track
	for {
		tok, err := nextToken(ctx, dec)
		if err != nil {
			return Track{}, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "name":
				name, err := readElementText(ctx, dec, tok)
				if err != nil {
					return Track{}, err
				}
				trk.Name = strings.TrimSpace(name)
			case "trkseg":
				seg, err := parseSegment(ctx, dec, tok)
				if err != nil {
					return Track{}, err
				}
				trk.Segments = append(trk.Segments, seg)
			default:
				if err := skipElement(ctx, dec, tok); err != nil {
					return Track{}, err
				}
			}
		case xml.EndElement:
			if sameElement(tok, start) {
				return trk, nil
			}
		}
	}
}

func parseSegment(ctx context.Context, dec *xml.Decoder, start xml.StartElement) (Segment, error) {
	var seg Segment
	for {
		tok, err := nextToken(ctx, dec)
		if err != nil {
			return Segment{}, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "name":
				name, err := readElementText(ctx, dec, tok)
				if err != nil {
					return Segment{}, err
				}
				seg.Name = strings.TrimSpace(name)
			case "trkpt":
				pt, err := parsePoint(ctx, dec, tok)
				if err != nil {
					return Segment{}, err
				}
				seg.Points = append(seg.Points, pt)
			default:
				if err := skipElement(ctx, dec, tok); err != nil {
					return Segment{}, err
				}
			}
		case xml.EndElement:
			if sameElement(tok, start) {
				return seg, nil
			}
		}
	}
}

func parsePoint(ctx context.Context, dec *xml.Decoder, start xml.StartElement) (Point, error) {
	pt, err := pointFromAttrs(start.Attr)
	if err != nil {
		return Point{}, err
	}
	if err := validatePoint(pt); err != nil {
		return Point{}, err
	}

	for {
		tok, err := nextToken(ctx, dec)
		if err != nil {
			return Point{}, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "ele":
				text, err := readElementText(ctx, dec, tok)
				if err != nil {
					return Point{}, err
				}
				ele, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
				if err != nil {
					return Point{}, err
				}
				pt.Ele = &ele
			case "time":
				text, err := readElementText(ctx, dec, tok)
				if err != nil {
					return Point{}, err
				}
				pt.Time = strings.TrimSpace(text)
			default:
				if err := skipElement(ctx, dec, tok); err != nil {
					return Point{}, err
				}
			}
		case xml.EndElement:
			if sameElement(tok, start) {
				return pt, nil
			}
		}
	}
}

func pointFromAttrs(attrs []xml.Attr) (Point, error) {
	var pt Point
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "lat":
			lat, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				return Point{}, err
			}
			pt.Lat = lat
		case "lon":
			lon, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				return Point{}, err
			}
			pt.Lon = lon
		}
	}
	return pt, nil
}

func validatePoint(pt Point) error {
	if pt.Lat < -90 || pt.Lat > 90 {
		return fmt.Errorf("invalid latitude %.6f: must be in [-90, 90]", pt.Lat)
	}
	if pt.Lon < -180 || pt.Lon > 180 {
		return fmt.Errorf("invalid longitude %.6f: must be in [-180, 180]", pt.Lon)
	}
	return nil
}

func readElementText(ctx context.Context, dec *xml.Decoder, start xml.StartElement) (string, error) {
	var text strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := nextToken(ctx, dec)
		if err != nil {
			return "", err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 1 {
				text.Write([]byte(tok))
			}
		}
	}
	return text.String(), nil
}

func skipElement(ctx context.Context, dec *xml.Decoder, start xml.StartElement) error {
	depth := 1
	for depth > 0 {
		tok, err := nextToken(ctx, dec)
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func nextToken(ctx context.Context, dec *xml.Decoder) (xml.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return tok, nil
}

func sameElement(end xml.EndElement, start xml.StartElement) bool {
	return end.Name.Local == start.Name.Local && end.Name.Space == start.Name.Space
}

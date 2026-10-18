package gpx

import (
	"bytes"
	"context"
	"encoding/xml"
	"strconv"
	"strings"
)

// pointDecoder accelerates only complete, ordinary track points at a segment
// boundary. Everything else stays with encoding/xml, including entities,
// namespaces, nested extensions, non-ASCII text and malformed input.
type pointDecoder struct {
	*xml.Decoder
	data []byte
	base int
}

func newPointDecoder(data []byte) *pointDecoder {
	return &pointDecoder{Decoder: xml.NewDecoder(bytes.NewReader(data)), data: data}
}

func (d *pointDecoder) appendPoints(ctx context.Context, points []Point, opts ParseOptions) ([]Point, error) {
	start := d.base + int(d.InputOffset())
	pos := start
	for {
		if len(points)%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		point, end, ok := scanPoint(d.data, skipXMLSpace(d.data, pos), opts)
		if !ok {
			break
		}
		if len(points) == cap(points) {
			// Grow geometrically without scanning the rest of the source to
			// count points first. A smaller growth factor repeatedly copies
			// large point arrays on dense tracks.
			grown := make([]Point, len(points), max(16, 2*cap(points)))
			copy(grown, points)
			points = grown
		}
		points = append(points, point)
		pos = end
	}
	if pos != start {
		// RawToken does not maintain an element stack. Restarting at the next
		// token preserves the surrounding parser's explicit nesting checks.
		d.base = pos
		d.Decoder = xml.NewDecoder(bytes.NewReader(d.data[pos:]))
	}
	return points, nil
}

func scanPoint(data []byte, pos int, opts ParseOptions) (Point, int, bool) {
	var point Point
	if !bytes.HasPrefix(data[pos:], []byte("<trkpt")) {
		return point, pos, false
	}
	pos += len("<trkpt")
	seen := 0
	for {
		next := skipXMLSpace(data, pos)
		if next == pos || next+3 >= len(data) {
			return Point{}, pos, false
		}
		pos = next
		name := string(data[pos : pos+3])
		bit := 0
		switch name {
		case "lat":
			bit = 1
		case "lon":
			bit = 2
		default:
			return Point{}, pos, false
		}
		if seen&bit != 0 {
			return Point{}, pos, false
		}
		pos = skipXMLSpace(data, pos+3)
		if pos >= len(data) || data[pos] != '=' {
			return Point{}, pos, false
		}
		pos = skipXMLSpace(data, pos+1)
		if pos >= len(data) || (data[pos] != '"' && data[pos] != '\'') {
			return Point{}, pos, false
		}
		quote := data[pos]
		pos++
		end := bytes.IndexByte(data[pos:], quote)
		if end < 0 {
			return Point{}, pos, false
		}
		value, err := strconv.ParseFloat(string(data[pos:pos+end]), 64)
		if err != nil {
			return Point{}, pos, false
		}
		if bit == 1 {
			point.Lat = value
		} else {
			point.Lon = value
		}
		seen |= bit
		pos += end + 1
		if seen == 3 {
			break
		}
	}
	if validatePoint(point) != nil {
		return Point{}, pos, false
	}
	pos = skipXMLSpace(data, pos)
	if bytes.HasPrefix(data[pos:], []byte("/>")) {
		return point, pos + 2, true
	}
	if pos >= len(data) || data[pos] != '>' {
		return Point{}, pos, false
	}
	pos++
	for {
		pos = skipXMLSpace(data, pos)
		if bytes.HasPrefix(data[pos:], []byte("</trkpt>")) {
			return point, pos + len("</trkpt>"), true
		}
		var name string
		switch {
		case bytes.HasPrefix(data[pos:], []byte("<ele>")):
			name = "ele"
		case bytes.HasPrefix(data[pos:], []byte("<time>")):
			name = "time"
		case bytes.HasPrefix(data[pos:], []byte("<extensions>")):
			name = "extensions"
		case bytes.HasPrefix(data[pos:], []byte("<extensions/>")):
			pos += len("<extensions/>")
			continue
		default:
			return Point{}, pos, false
		}
		pos += len(name) + 2
		if name == "extensions" {
			end := skipXMLSpace(data, pos)
			if !bytes.HasPrefix(data[end:], []byte("</extensions>")) {
				return Point{}, pos, false
			}
			pos = end + len("</extensions>")
			continue
		}
		end := pos
		for end < len(data) && data[end] != '<' {
			c := data[end]
			// The decoder owns entity expansion, UTF-8/XML character validity,
			// CR normalization and the forbidden CDATA closing sequence.
			if c == '&' || c == '>' || c >= 0x7f || (c < 0x20 && c != '\n' && c != '\t') {
				return Point{}, pos, false
			}
			end++
		}
		if !bytes.HasPrefix(data[end:], []byte("</"+name+">")) {
			return Point{}, pos, false
		}
		switch name {
		case "ele":
			if opts.KeepEle {
				value, err := strconv.ParseFloat(strings.TrimSpace(string(data[pos:end])), 64)
				if err != nil {
					return Point{}, pos, false
				}
				point.Ele = &value
			}
		case "time":
			if opts.KeepTime {
				point.Time = strings.TrimSpace(string(data[pos:end]))
			}
		}
		pos = end + len(name) + 3
	}
}

func skipXMLSpace(data []byte, pos int) int {
	for pos < len(data) {
		switch data[pos] {
		case ' ', '\t', '\n', '\r':
			pos++
		default:
			return pos
		}
	}
	return pos
}

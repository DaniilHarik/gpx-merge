package gpx

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPointDecoderCancellation(t *testing.T) {
	data := []byte(strings.Repeat(`<trkpt lat="58" lon="24"/>`, 1024))
	ctx := &parseStepCancelContext{Context: context.Background(), cancelOnCall: 2}
	_, err := newPointDecoder(data).appendPoints(ctx, nil, ParseOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ordinary point parsing returned %v, want context.Canceled", err)
	}
}

// Exercise the file parser against the original XML-token path, including
// transitions between ordinary points and XML that needs the general decoder.
func TestParsePointCompatibility(t *testing.T) {
	points := []string{
		`<trkpt lat="58.123" lon="24.456"><ele>10.5</ele><time>2024-01-01T00:00:00Z</time><extensions> </extensions></trkpt>`,
		"<trkpt lat=\"58\" lon=\"24\">\r\n<extensions>\r\n\t</extensions>\r\n</trkpt>",
		`<trkpt lon='2.4e1' lat = '5.8e1' />`,
		`<trkpt lat="58" lon="24"><ele>invalid</ele><time> A &amp; B </time></trkpt>`,
		`<trkpt lat="58" lon="24"><extensions><nested><trkpt lat="1" lon="2"/></nested></extensions></trkpt>`,
		`<x:trkpt xmlns:x="urn:test" lat="58" lon="24"><x:ele>3</x:ele></x:trkpt>`,
		`<trkpt lat="58" lon="24"><time><![CDATA[A&B]]></time></trkpt>`,
		`<trkpt lat="58" lon="24"><ele/><time/></trkpt>`,
		`<trkpt lat="58" lon="24"><time>A<b>B</b>C</time></trkpt>`,
		"<trkpt lat=\"58\" lon=\"24\"><time>A\r\nB</time></trkpt>",
		`<trkpt lat="58" lon="24"><ele>1</ele><ele>2</ele><time>A</time><time>B</time></trkpt>`,
		`<trkpt lat="&#53;8" lon="24"/>`,
		`<trkpt lat="91" lon="24"/>`,
		`<trkpt lat="58" lon="24"><extensions></wrong></extensions></trkpt>`,
		`<trkpt lat="58" lon="24"><time>A &unknown;</time></trkpt>`,
		"<trkpt lat=\"58\" lon=\"24\"><time>\x00</time></trkpt>",
		`<trkpt lat="58" lon="24"><time>]]></time></trkpt>`,
		`<trkpt lat="58" lon="24"><ele>2</ele>`,
	}
	for _, point := range points {
		for _, opts := range []ParseOptions{{}, {KeepEle: true}, {KeepTime: true}, {KeepEle: true, KeepTime: true}} {
			doc := `<gpx><trk><name>Compatibility</name><trkseg>` +
				`<trkpt lat="1" lon="2"/>` + point + `<trkpt lat="3" lon="4"/>` +
				`</trkseg></trk></gpx>`
			want, wantErr := parseGPX(context.Background(), xml.NewDecoder(strings.NewReader(doc)), opts)
			path := writeGPXFile(t, t.TempDir(), "compat.gpx", doc)
			got, gotErr := ParseFileWithOptions(context.Background(), path, "compat.gpx", opts)
			if (gotErr == nil) != (wantErr == nil) || (gotErr == nil && !reflect.DeepEqual(got, want)) {
				t.Fatalf("point=%q opts=%+v\ngot %v, %v\nwant %v, %v", point, opts, got, gotErr, want, wantErr)
			}
		}
	}
}

func FuzzParsePointCompatibility(f *testing.F) {
	for _, seed := range []string{
		`<trkpt lat="58" lon="24"><ele>1</ele><time>now</time><extensions/></trkpt>`,
		"<trkpt lat='58' lon='24'>\r\n<extensions>\r\n</extensions></trkpt>",
		`<trkpt lat="NaN" lon="24"/>`,
		`<trkpt lat="58" lon="24"><extensions><x/></extensions></trkpt>`,
		`<trkpt lat="58" lon="24"><time>&amp;</time></trkpt>`,
		`<!-- comment --><trkpt lon="24" lat="58"/>`,
	} {
		f.Add(seed, true, true)
		f.Add(seed, false, false)
	}
	f.Fuzz(func(t *testing.T, point string, keepEle, keepTime bool) {
		doc := []byte(`<gpx><trk><trkseg><trkpt lat="1" lon="2"/>` + point +
			`<trkpt lat="3" lon="4"/></trkseg></trk></gpx>`)
		opts := ParseOptions{KeepEle: keepEle, KeepTime: keepTime}
		want, wantErr := parseGPX(context.Background(), xml.NewDecoder(bytes.NewReader(doc)), opts)
		got, gotErr := parseGPX(context.Background(), newPointDecoder(doc), opts)
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("errors differ: fast=%v standard=%v", gotErr, wantErr)
		}
		if gotErr == nil {
			// XML marshaling also compares NaN values, which are deliberately
			// left to the existing coordinate-validation contract.
			gotXML, err := xml.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			wantXML, err := xml.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotXML, wantXML) {
				t.Fatalf("tracks differ: fast=%s standard=%s", gotXML, wantXML)
			}
		}
	})
}

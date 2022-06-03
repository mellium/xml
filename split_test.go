// Copyright 2021 The Mellium Contributors.
// Use of this source code is governed by the BSD 2-clause
// license that can be found in the LICENSE file.

package xml_test

import (
	"bufio"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"

	. "mellium.im/xml"
)

const (
	cdataStart = "<![CDATA["
	cdataEnd   = "]]>"
)

var splitTestCases = []struct {
	in string
}{
	0: {},
	1: {
		in: "test<a/>",
	},
	2: {in: "test<a></a>"},
	3: {in: `<![CDATA[ ..>. ]]>`},
	4: {in: `<a test=">"></a>`},
	5: {in: `<a test='>'></a>`},
	6: {in: `<stream:stream xmlns='jabber:server' xmlns:stream='http://etherx.jabber.org/streams' xmlns:db='jabber:server:dialback' version='1.0' to='example.org' from='example.com' xml:lang='en'>
<a/><b>inside b before c<c>inside c</c></b>
<q>bla<![CDATA[<this>is</not><xml/>]]>bloo</q>
<x><![CDATA[ lol</x> ]]></x>
<z><x><![CDATA[ lol</x> ]]></x></z>
<a a='![CDATA['/>
<x a='/>'>This is going to be fun.</x>
<z><x a='/>'>This is going to be fun.</x></z>
<d></d><e><![CDATA[what]>]]]]></e></stream:stream>`},
}

func TestSplit(t *testing.T) {
	for i, tc := range splitTestCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			d := xml.NewDecoder(strings.NewReader(tc.in))
			scan := bufio.NewScanner(strings.NewReader(tc.in))
			scan.Split(Split)

			for scan.Scan() {
				tokText := scan.Text()
				tok, err := d.Token()
				if err != nil {
					t.Fatalf("error decoding token %q: %v", tokText, err)
				}
				t.Logf("Split tok: %q, %T(%[2]v)", string(tokText), tok)
				switch typedTok := tok.(type) {
				case xml.Comment:
					if string(typedTok) != tokText {
						t.Fatalf("wrong comment: want=%q, got=%q", typedTok, tokText)
					}
				case xml.CharData:
					trimText := strings.TrimPrefix(tokText, cdataStart)
					trimText = strings.TrimSuffix(trimText, cdataEnd)
					if string(typedTok) != trimText {
						t.Fatalf("wrong chardata: want=%q, got=%q", typedTok, trimText)
					}
				}
				if strings.HasSuffix(tokText, "/>") {
					tok, err = d.Token()
					if err != nil {
						t.Fatalf("error decoding token %q: %v", tokText, err)
					}
					if _, ok := tok.(xml.EndElement); !ok {
						t.Fatalf("expected xml.EndElement, but got %T (%[1]v)", tok)
					}
				}
			}
			if err := scan.Err(); err != nil {
				t.Fatalf("unexpected error while scanning: %v", err)
			}
			tok, err := d.Token()
			if (err != io.EOF && err != nil) || tok != nil {
				t.Fatalf("unexpected extra %T or error decoded: %[1]v, %v", tok, err)
			}
		})
	}
}

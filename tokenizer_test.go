// Copyright 2022 The Mellium Contributors.
// Use of this source code is governed by the BSD 2-clause
// license that can be found in the LICENSE file.

package xml_test

import (
	"encoding/xml"
	"reflect"
	"strconv"
	"strings"
	"testing"

	. "mellium.im/xml"
)

var tokenizerTestCases = []struct {
	in string
}{
	// TODO: syntax error with target/inst? I can't remember what character caused
	// it.
	0: {in: `<?inst target?><?inst tar get ?><?inst>target?>`},
	// TODO: <!--test---> syntax error?
	1: {in: `<!--test--><!-- test --><!-- test- -->`},
	2: {in: `<!dir>, <! test >`},
	3: {in: `<test></test><foo bar="baz"></foo><foo2 bar="baz" bar="boz"></foo2>`},
	4: {in: `<?xml version="1.0" encoding="UTF-8"?>
<?Target Instruction?>
<root>
</root>
`},
	5: {in: `<foo/><bar
	/>`},
	6: {in: `<baz xmlns="g" g:test="yes"><bar xmlns:g="me"><foo xmlns:h="hi" h:attr="boo" g:attr="my"/></bar></baz>`},
	7: {in: `<a:href xmlns:a="test"></a:href>`},
	8: {in: `<foo xmlns="foo"><bar a="b"/></foo>`},
	9: {in: `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN"
  "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<body xmlns:foo="ns1" xmlns="ns2" xmlns:tag="ns3" ` +
		"\r\n\t" + `  >
  <hello lang="en">World &lt;&gt;&apos;&quot; &#x767d;&#40300;翔</hello>
  <query>&何; &is-it;</query>
  <goodbye />
  <outer foo:attr="value" xmlns:tag="ns4">
    <inner/>
  </outer>
  <tag:name>
    <![CDATA[Some text here.]]>
  </tag:name>
</body><!-- missing final newline -->`},
}

func TestTokenize(t *testing.T) {
	for i, tc := range tokenizerTestCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			d := xml.NewDecoder(strings.NewReader(tc.in))
			td := NewTokenizer(strings.NewReader(tc.in))

			for {
				ttok, terr := td.Token()
				tok, err := d.Token()
				if err != terr {
					t.Fatalf("mismatched error decoding: want=%v, got=%v", err, terr)
				}
				if !reflect.DeepEqual(ttok, tok) {
					t.Fatalf("mismatched token:\nwant=%T(%+[1]v),\n got=%[2]T(%+[2]v)", tok, ttok)
				}
				if err != nil || terr != nil {
					return
				}
			}
		})
	}
}

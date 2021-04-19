// Copyright 2021 The Mellium Contributors.
// Use of this source code is governed by the BSD 2-clause
// license that can be found in the LICENSE file.

package xml_test

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"mellium.im/xml"
)

// Use a Scanner to split a byte stream on possible XML tokens.
func ExampleScanner() {
	scanner := bufio.NewScanner(strings.NewReader(`<root>
  <foo test="split me"/>
</root>`))
	scanner.Split(xml.Split)
	for scanner.Scan() {
		fmt.Printf("%q ", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "shouldn't see an error scanning a string")
	}
	// Output:
	// "<root>" "\n  " "<foo test=\"split me\"/>" "\n" "</root>"
}

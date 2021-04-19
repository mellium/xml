// Copyright 2021 The Mellium Contributors.
// Use of this source code is governed by the BSD 2-clause
// license that can be found in the LICENSE file.

// Package xml contains experimental XML functionality.
//
// This package may be deprecated or removed at any time.
package xml // import "mellium.im/xml"

import (
	"bytes"
	"io"
)

var (
	cdataStart = []byte("<![CDATA[")
	cdataEnd   = []byte("]]>")
)

// Split is a bufio.SplitFunc that splits on XML tokens.
func Split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 && atEOF {
		return 0, nil, io.EOF
	}

	switch {
	case bytes.HasPrefix(data, cdataStart):
		return splitCData(data, atEOF)
	case data[0] != '<':
		return splitCharData(data, atEOF)
	}
	return splitOther(data, atEOF)
}

func splitCData(data []byte, atEOF bool) (int, []byte, error) {
	idx := bytes.Index(data, cdataEnd)
	if idx == -1 {
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}

	return idx + len(cdataEnd), data[:idx+len(cdataEnd)], nil
}

func splitCharData(data []byte, atEOF bool) (int, []byte, error) {
	idx := bytes.IndexByte(data, '<')
	if idx == -1 {
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}

	return idx, data[:idx], nil
}

func splitOther(data []byte, atEOF bool) (int, []byte, error) {
	var startQuote byte
	for i, b := range data {
		if startQuote != 0 {
			if b == startQuote {
				startQuote = 0
			}
			continue
		}

		switch b {
		case '"', '\'':
			startQuote = b
		case '>':
			return i + 1, data[:i+1], nil
		}
	}

	if atEOF {
		// TODO: is this an invalid token if it starts with an unescaped '<'?
		// Should we return an error?
		return len(data), data, nil
	}
	return 0, nil, nil
}

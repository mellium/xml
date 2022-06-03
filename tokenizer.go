// Copyright 2022 The Mellium Contributors.
// Use of this source code is governed by the BSD 2-clause
// license that can be found in the LICENSE file.

package xml

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

const xmlURL = "http://www.w3.org/XML/1998/namespace"

// Various types re-used from the standard library XML package, aliased here so
// that both packages don't need to be imported.
type (
	Attr                 = xml.Attr
	CharData             = xml.CharData
	Comment              = xml.Comment
	Directive            = xml.Directive
	EndElement           = xml.EndElement
	Name                 = xml.Name
	ProcInst             = xml.ProcInst
	StartElement         = xml.StartElement
	TokenReader          = xml.TokenReader
	Token                = xml.Token
	Marshaler            = xml.Marshaler
	Encoder              = xml.Encoder
	TagPathError         = xml.TagPathError
	Decoder              = xml.Decoder
	Unmarshaler          = xml.Unmarshaler
	UnmarshalerAttr      = xml.UnmarshalerAttr
	MarshalerAttr        = xml.MarshalerAttr
	UnsupportedTypeError = xml.UnsupportedTypeError
	SyntaxError          = xml.SyntaxError
)

var (
	MarshalIndent   = xml.MarshalIndent
	Marshal         = xml.Marshal
	Unmarshal       = xml.Unmarshal
	NewEncoder      = xml.NewEncoder
	NewTokenDecoder = xml.NewTokenDecoder
	CopyToken       = xml.CopyToken
	EscapeText      = xml.EscapeText
	HTMLAutoClose   = xml.HTMLAutoClose
	HTMLEntity      = xml.HTMLEntity
)

var errEarlyEOF = &SyntaxError{Msg: "early EOF"}

// NewDecoder creates a new XML parser reading from r.
// If r does not implement io.ByteReader, NewDecoder will do its own buffering.
func NewDecoder(r io.Reader) *Decoder {
	return NewTokenDecoder(NewTokenizer(r))
}

// Tokenizer splits a reader into XML tokens without performing any verification
// or namespace resolution on those tokens.
type Tokenizer struct {
	r          io.ByteReader
	foundStart bool
	selfClose  *xml.Name
	prefixes   []map[string]string
	spaces     []string
}

// NewTokenizer creates a new XML parser reading from r.
// If r does not implement io.ByteReader, NewDecoder will do its own buffering.
func NewTokenizer(r io.Reader) *Tokenizer {
	t := &Tokenizer{}
	if br, ok := r.(io.ByteReader); ok {
		t.r = br
	} else {
		t.r = bufio.NewReader(r)
	}
	return t
}

// Token returns the next XML token in the input stream.
// At the end of the input stream, Token returns nil, io.EOF.
func (t *Tokenizer) Token() (Token, error) {
	if t.selfClose != nil {
		name := *t.selfClose
		t.selfClose = nil
		return xml.EndElement{Name: name}, nil
	}
	var b byte
	var err error
	if t.foundStart {
		b = '<'
		t.foundStart = false
	} else {
		b, err = t.r.ReadByte()
		if err != nil {
			return nil, err
		}
	}

	// We found a CharData. Read until we consume another '<'.
	if b != '<' {
		// TODO: reuse buf
		buf := []byte{b}
		return decodeCharData(t, buf)
	}

	// We found a '<', figure out what it is.
	b, err = t.r.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errEarlyEOF
		}
		return nil, err
	}
	switch b {
	case '!':
		// Directive or comment
		// TODO: reuse buffer
		var buf []byte
		b, err := t.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errEarlyEOF
			}
			return nil, err
		}
		buf = append(buf, b)
		if b == '-' {
			b, err = t.r.ReadByte()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil, errEarlyEOF
				}
				return nil, err
			}
			buf = append(buf, b)
			if b == '-' {
				buf = buf[:0]
				return decodeComment(t, buf)
			} else {
				return nil, &SyntaxError{Msg: "invalid sequence <!- not part of <!--"}
			}
		}
		return decodeDirective(t, buf)
	case '?':
		// ProcInst <?target inst?>
		// TODO: reuse buffer
		tok, err := decodeProcInst(t, nil)
		if err != nil {
			return nil, err
		}
		return tok, nil
	case '/':
		return decodeEndElement(t)
	}
	// StartElement, or self-closing
	return decodeStartElement(t, b)
}

func decodeStartElement(t *Tokenizer, b byte) (StartElement, error) {
	t.spaces = append(t.spaces, "")
	// TODO: defer make until we actually find a prefix?
	t.prefixes = append(t.prefixes, make(map[string]string))
	// TODO: check for space as sep?
	name, sep, def, err := decodeName(t, b, false)
	if err != nil {
		return StartElement{}, err
	}
	// We use an empty array instead of nil to match the behavior of encoding/xml.
	attr := []Attr{}
	for {
		// If we reach the end, don't decode any more attributes.
		switch sep {
		case 0x20, 0x9, 0xD, 0xA:
			// Consume any spaces between the name and attributes.
			sep, err = t.r.ReadByte()
			if err != nil {
				return StartElement{}, err
			}
			continue
		case '/':
			t.selfClose = &name
			sep, err = t.r.ReadByte()
			if err != nil {
				return StartElement{}, err
			}
			if sep != '>' {
				return StartElement{}, fmt.Errorf("xml: expected > to end the element, got %q", string(sep))
			}
			fallthrough
		case '>':
			return StartElement{Name: name, Attr: attr}, nil
		}

		// Decode the attribute we found.
		a, err := decodeAttr(t, sep)
		if err != nil {
			return StartElement{}, err
		}
		sep, err = t.r.ReadByte()
		if err != nil {
			return StartElement{}, err
		}
		if a.Name.Local != "" {
			attr = append(attr, a)
		}
		switch {
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			name.Space = a.Value
			def = true
			t.spaces[len(t.spaces)-1] = a.Value
		case a.Name.Space == "xmlns":
			t.prefixes[len(t.prefixes)-1][a.Name.Local] = a.Value
			if !def && name.Space != "" && name.Space == a.Name.Local {
				name.Space = a.Value
			}
		}
	}
	return StartElement{Name: name, Attr: attr}, nil
}

func decodeEndElement(t *Tokenizer) (EndElement, error) {
	defer func() {
		if len(t.prefixes) > 0 {
			t.prefixes = t.prefixes[:len(t.prefixes)-1]
		}
		if len(t.spaces) > 0 {
			t.spaces = t.spaces[:len(t.spaces)-1]
		}
	}()
	// TODO: check for space as sep?
	name, _, _, err := decodeName(t, 0, false)
	if err != nil {
		return EndElement{}, err
	}
	return EndElement{Name: name}, nil
}

func decodeName(t *Tokenizer, b byte, attr bool) (Name, byte, bool, error) {
	// Set to the previous default namespace. If we find a new namespace this will
	// be overwritten later.
	var space string
	// Attributes don't get the default namespace.
	if !attr {
		for i := len(t.spaces) - 1; i >= 0; i-- {
			space = t.spaces[i]
			if space != "" {
				break
			}
		}
	}

	// TODO: reuse builder
	var foundSep bool
	var rawFirst, rawSecond strings.Builder
	if b != 0 {
		/* #nosec */
		rawFirst.WriteByte(b)
	}

	for {
		b, err := t.r.ReadByte()
		if err != nil {
			return Name{}, 0, false, err
		}
		if !isNameByte(b) {
			if foundSep {
				space = rawFirst.String()
				// Go backwards up the stack looking for a prefix definition. If we find
				// one, replace the namespace with it
				for i := len(t.prefixes) - 1; i >= 0; i-- {
					if resolvedSpace := t.prefixes[i][space]; resolvedSpace != "" {
						space = resolvedSpace
						break
					}
				}
				return Name{Space: space, Local: rawSecond.String()}, b, false, nil
			} else {
				return Name{Space: space, Local: rawFirst.String()}, b, space != "", nil
			}
		}
		if b == ':' {
			foundSep = true
			continue
		}
		if foundSep {
			/* #nosec */
			rawSecond.WriteByte(b)
		} else {
			/* #nosec */
			rawFirst.WriteByte(b)
		}
	}
}

func decodeAttr(t *Tokenizer, b byte) (Attr, error) {
	name, sep, _, err := decodeName(t, b, true)
	if err != nil {
		return Attr{}, err
	}
	if sep != '=' {
		return Attr{}, fmt.Errorf("xml: bad attribute separator %q", string(sep))
	}
	b, err = t.r.ReadByte()
	if err != nil {
		return Attr{}, err
	}
	if b != '\'' && b != '"' {
		return Attr{}, fmt.Errorf("xml: expected quoted attribute value")
	}
	quote := b
	// Get the value
	// TODO: reuse builder
	var raw strings.Builder
	for {
		b, err = t.r.ReadByte()
		if err != nil {
			return Attr{}, err
		}
		// TODO: what characters are valid in a name?
		if b == quote {
			return Attr{
				Name:  name,
				Value: raw.String(),
			}, nil
		}
		raw.WriteByte(b)
	}
}

func decodeDirective(t *Tokenizer, dir []byte) (Directive, error) {
	for {
		b, err := t.r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == '>' {
			return Directive(dir), nil
		}
		dir = append(dir, b)
	}
}

func decodeComment(t *Tokenizer, comment []byte) (Comment, error) {
	var found uint8
	for {
		b, err := t.r.ReadByte()
		if err != nil {
			return nil, err
		}
		switch {
		case b == '-':
			found++
			continue
		case b == '>' && found > 1:
			return Comment(comment), nil
		default:
			for i := uint8(0); i < found; i++ {
				comment = append(comment, '-')
			}
			found = 0
		}
		comment = append(comment, b)
	}
}

func decodeProcInst(t *Tokenizer, inst []byte) (ProcInst, error) {
	var (
		foundSpace bool
		foundEnd   bool
		target     strings.Builder
	)
	for {
		b, err := t.r.ReadByte()
		if err != nil {
			return ProcInst{}, err
		}
		switch b {
		case '>':
			// If we found ?>, this is the end and we can return the token.
			if foundEnd {
				return ProcInst{Target: target.String(), Inst: inst}, nil
			}
			foundSpace = true
		case '?':
			foundEnd = true
			foundSpace = true
			continue
		case 0x20, 0x9, 0xD, 0xA:
			if !foundSpace {
				foundSpace = true
				continue
			}
		}
		if foundSpace {
			if target.Len() == 0 {
				return ProcInst{}, &SyntaxError{Msg: "xml: expected target name after <?"}
			}
			if foundEnd {
				inst = append(inst, '?')
				foundEnd = false
			}
			inst = append(inst, b)
		} else {
			/* #nosec */
			target.WriteByte(b)
		}
	}
}

func decodeCharData(t *Tokenizer, buf []byte) (CharData, error) {
	for {
		b, err := t.r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == '<' {
			t.foundStart = true
			break
		}
		buf = append(buf, b)
	}
	// TODO: unescape bytes, or leave them and will the decoder do it?
	return CharData(buf), nil
}

func isSpace(b byte) bool {
	return b == 0x20 || b == 0x9 || b == 0xD || b == 0xA
}

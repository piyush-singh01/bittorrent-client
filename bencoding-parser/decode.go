package bencodingParser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"unicode"
)

const (
	StringType = iota
	IntegerType
	ListType
	DictionaryType
)

func getBencodeType(data []byte, pos int) int {
	if pos < 0 || pos >= len(data) {
		return -1
	}

	switch {
	case data[pos] == 'i':
		return IntegerType
	case data[pos] == 'l':
		return ListType
	case data[pos] == 'd':
		return DictionaryType
	case unicode.IsDigit(rune(data[pos])):
		return StringType
	default:
		return -1
	}
}

func ParseInt(data []byte, startPos int) (bencode *Bencode, endPos int, err error) {
	if startPos < 0 || startPos >= len(data) {
		err = fmt.Errorf("position %d is invalid", startPos)
		return
	}

	if data[startPos] != 'i' {
		err = fmt.Errorf("error while parsing integer at position %d: expected 'i'", startPos)
		return
	}

	endPos = bytes.IndexByte(data[startPos:], 'e')
	if endPos == -1 {
		err = fmt.Errorf("missing 'e' terminator for integer starting at pos %d", startPos)
		return
	}
	endPos += startPos

	parsedInt, parseErr := strconv.Atoi(string(data[startPos+1 : endPos]))
	if parseErr != nil {
		err = fmt.Errorf("error %v converting to integer at pos %d", parseErr, startPos)
		return
	}

	bencode = NewBencodeFromBInt(NewBencodeInt(parsedInt))
	endPos++
	return bencode, endPos, err
}

func ParseString(data []byte, startPos int) (bencode *Bencode, endPos int, err error) {
	if startPos < 0 || startPos >= len(data) {
		err = fmt.Errorf("position %d is invalid", startPos)
		return
	}

	if data[startPos] < '0' || data[startPos] > '9' {
		err = fmt.Errorf("error while parsing string at pos %d: expected numeric", startPos)
		return
	}

	colonIdx := bytes.IndexByte(data[startPos:], ':')
	if colonIdx == -1 {
		err = fmt.Errorf("missing ':' delimiter for string length at pos %d", startPos)
		return
	}
	colonIdx += startPos

	strLen, parseErr := strconv.Atoi(string(data[startPos:colonIdx]))
	if parseErr != nil {
		err = fmt.Errorf("error: %v while converting integer to string at pos %d", parseErr, startPos)
		return
	}

	endPos = colonIdx + strLen + 1
	if endPos > len(data) {
		err = fmt.Errorf("string length %d exceeds data bounds starting at pos %d", strLen, startPos)
		return
	}

	bencode = NewBencodeFromBString(NewBencodeString(string(data[colonIdx+1 : endPos])))
	return bencode, endPos, err
}

func ParseList(data []byte, startPos int) (bencode *Bencode, endPos int, err error) {
	if startPos < 0 || startPos >= len(data) {
		err = fmt.Errorf("position %d is invalid", startPos)
		return
	}

	if data[startPos] != 'l' {
		err = fmt.Errorf("error while parsing list at position %d: expected 'l'", startPos)
		return
	}

	bencodeList := NewBencodeList()
	pos := startPos + 1
	for data[pos] != 'e' {
		bencodeType := getBencodeType(data, pos)
		if bencodeType < 0 {
			return nil, pos, fmt.Errorf("unhandled bencode type at position %d", pos)
		}
		var bencodeCurr *Bencode

		switch bencodeType {
		case StringType:
			bencodeCurr, pos, err = ParseString(data, pos)
		case IntegerType:
			bencodeCurr, pos, err = ParseInt(data, pos)
		case ListType:
			bencodeCurr, pos, err = ParseList(data, pos)
		case DictionaryType:
			bencodeCurr, pos, err = ParseDictionary(data, pos)
		}

		bencodeList.Add(bencodeCurr)
	}
	bencode = NewBencodeFromBList(bencodeList)

	return bencode, pos + 1, err
}

func ParseDictionary(data []byte, startPos int) (bencode *Bencode, endPos int, err error) {
	if startPos < 0 || startPos >= len(data) {
		err = fmt.Errorf("position %d is invalid", startPos)
		return
	}

	if data[startPos] != 'd' {
		err = fmt.Errorf("error while parsing list at position %d: expected 'd'", startPos)
		return
	}

	bencodeDictionary := NewBencodeDict()
	pos := startPos + 1
	for data[pos] != 'e' {
		bencodeTypeKey := getBencodeType(data, pos)
		if bencodeTypeKey != StringType {
			err = fmt.Errorf("error at pos %d: key is not a string", pos)
			return
		}

		var bencodeKey *Bencode
		bencodeKey, pos, err = ParseString(data, pos)

		bencodeTypeValue := getBencodeType(data, pos)
		if bencodeTypeValue < 0 {
			return nil, pos, fmt.Errorf("unhandled bencode type at position %d", pos)
		}

		var bencodeValue *Bencode
		switch bencodeTypeValue {
		case StringType:
			bencodeValue, pos, err = ParseString(data, pos)
		case IntegerType:
			bencodeValue, pos, err = ParseInt(data, pos)
		case ListType:
			bencodeValue, pos, err = ParseList(data, pos)
		case DictionaryType:
			bencodeValue, pos, err = ParseDictionary(data, pos)
		}

		bencodeDictionary.Put(bencodeKey, bencodeValue)
	}
	bencode = NewBencodeFromBDict(bencodeDictionary)
	return bencode, pos + 1, err
}

func ParseBencodeTorrentFile(reader io.Reader) (bencode *Bencode, err error) {
	fileContent, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("error reading file %v", err)
	}

	bencode, _, err = ParseDictionary(fileContent, 0)
	if err != nil {
		err = errors.New("parsing error: " + err.Error())
	}
	return bencode, err
}

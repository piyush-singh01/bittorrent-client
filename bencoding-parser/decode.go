package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
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
	case rune(data[pos]) == 'i':
		return IntegerType
	case rune(data[pos]) == 'l':
		return ListType
	case rune(data[pos]) == 'd':
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

	if rune(data[startPos]) != 'i' {
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
	return bencode, endPos + 1, nil
}

func ParseString(data []byte, startPos int) (bencode *Bencode, endPos int, err error) {
	if startPos < 0 || startPos >= len(data) {
		err = fmt.Errorf("position %d is invalid", startPos)
		return
	}

	if !unicode.IsDigit(rune(data[startPos])) {
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

	endPos = colonIdx + strLen
	bencode = NewBencodeFromBString(NewBencodeString(string(data[colonIdx+1 : endPos+1])))
	return bencode, endPos + 1, nil
}

func ParseList(data []byte, startPos int) (bencode *Bencode, endPos int, err error) {
	if startPos < 0 || startPos >= len(data) {
		err = fmt.Errorf("position %d is invalid", startPos)
		return
	}

	if rune(data[startPos]) != 'l' {
		err = fmt.Errorf("error while parsing list at position %d: expected 'l'", startPos)
		return
	}

	bencodeList := NewBencodeList()
	pos := startPos + 1
	for rune(data[pos]) != 'e' {
		bencodeType := getBencodeType(data, pos)
		var bencodeCurr *Bencode

		switch bencodeType {
		case StringType:
			bencodeCurr, pos, err = ParseString(data, pos)
		case IntegerType:
			bencodeCurr, pos, err = ParseInt(data, pos)
		case ListType:
			bencodeCurr, pos, err = ParseList(data, pos)
		default:
			panic("unhandled default case")
		}

		if err != nil {
			panic("something went wrong in parsing list") // todo
		}

		bencodeList.AddToBencodeList(bencodeCurr)
	}
	bencode = NewBencodeFromBList(bencodeList)

	return bencode, pos + 1, nil
}

func ParseDictionary(data []byte, startPos int) (bencode *Bencode, endPos int, err error) {
	if startPos < 0 || startPos >= len(data) {
		err = fmt.Errorf("position %d is invalid", startPos)
		return
	}

	if rune(data[startPos]) != 'd' {
		err = fmt.Errorf("error while parsing list at position %d: expected 'd'", startPos)
		return
	}

	bencodeDictionary := NewBencodeDict()
	pos := startPos + 1
	for rune(data[pos]) != 'e' {
		bencodeTypeKey := getBencodeType(data, pos)
		if bencodeTypeKey != StringType {
			err = fmt.Errorf("error at pos %d: key is not a string", pos)
			return
		}

		var bencodeKey *Bencode
		bencodeKey, pos, err = ParseString(data, pos)
		if err != nil {
			panic("something went wrong in parsing dictionary") // todo
		}

		bencodeTypeValue := getBencodeType(data, pos)
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
		default:
			panic("unhandled default case")
		}

		bencodeDictionary.PutInDictionary(bencodeKey, bencodeValue)
	}
	bencode = NewBencodeFromBDict(bencodeDictionary)
	return bencode, pos + 1, nil
}

func main() {

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run decode.go <filename>")
		return
	}

	fileContent, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("error reading file %v", err)
	}

	bencode, endPos, err := ParseDictionary(fileContent, 0)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(fileContent))
	fmt.Println(bencode)
	fmt.Println(endPos)

}

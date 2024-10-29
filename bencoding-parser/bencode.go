package main

import (
	"fmt"
	"strings"
)

type BencodeString string
type BencodeInt int
type BencodeList []Bencode
type BencodeDict map[BencodeString]Bencode

func NewBencodeString(s string) *BencodeString {
	bencodeString := BencodeString(s)
	return &bencodeString
}

func NewBencodeInt(v int) *BencodeInt {
	BInt := BencodeInt(v)
	return &BInt
}

func NewBencodeList() *BencodeList {
	var BList BencodeList
	return &BList
}

func NewBencodeDict() *BencodeDict {
	var BDict BencodeDict = make(map[BencodeString]Bencode)
	return &BDict
}

func (bs *BencodeString) String() string {
	return fmt.Sprint(*bs)
}

func (bi *BencodeInt) String() string {
	return fmt.Sprint(*bi)
}

func (bl *BencodeList) String() string {
	var res []string
	for _, ele := range *bl {
		res = append(res, fmt.Sprint(&ele))
	}
	return fmt.Sprint(res)
}

func (bd *BencodeDict) String() string {
	return stringify(bd, 0)
}

func stringify(bd *BencodeDict, indentLvl int) string {
	indent := strings.Repeat("\t", indentLvl+1)
	bracesIndent := strings.Repeat("\t", indentLvl)
	res := bracesIndent + "{\n"
	for key, value := range *bd {
		if value.getBencodeType() == DictionaryType {
			res += fmt.Sprintln(indent + fmt.Sprint(&key) + ": " + stringify(value.BDict, indentLvl+1))

		} else {
			res += fmt.Sprintln(indent + fmt.Sprint(&key) + ": " + fmt.Sprint(&value))
		}
	}
	res += bracesIndent + "}\n"
	return res
}

func (bl *BencodeList) AddToBencodeList(b *Bencode) {
	*bl = append(*bl, *b)
}

func (bd *BencodeDict) PutInDictionary(key *Bencode, value *Bencode) {
	(*bd)[*(key.BString)] = *value
}

type Bencode struct {
	BString *BencodeString
	BInt    *BencodeInt
	BList   *BencodeList
	BDict   *BencodeDict
}

func (b *Bencode) String() string {
	if b.BString != nil {
		return fmt.Sprint(b.BString)
	} else if b.BInt != nil {
		return fmt.Sprint(b.BInt)
	} else if b.BList != nil {
		return fmt.Sprint(b.BList)
	} else if b.BDict != nil {
		return fmt.Sprint(b.BDict)
	}
	return ""
}

func (b *Bencode) getBencodeType() int {
	if b.BString != nil {
		return StringType
	} else if b.BInt != nil {
		return IntegerType
	} else if b.BList != nil {
		return ListType
	} else if b.BDict != nil {
		return DictionaryType
	}
	return -1
}

func NewBencodeFromBString(bs *BencodeString) *Bencode {
	return &Bencode{
		BString: bs,
	}
}

func NewBencodeFromBInt(bi *BencodeInt) *Bencode {
	return &Bencode{
		BInt: bi,
	}
}

func NewBencodeFromBList(bl *BencodeList) *Bencode {
	return &Bencode{
		BList: bl,
	}
}

func NewBencodeFromBDict(bd *BencodeDict) *Bencode {
	return &Bencode{
		BDict: bd,
	}
}

func (b *Bencode) panicIfMultipleAssignment() {
	count := 0
	if b.BString != nil {
		count++
	}
	if b.BInt != nil {
		count++
	}
	if b.BList != nil {
		count++
	}
	if b.BDict != nil {
		count++
	}
	if count > 1 {
		panic("error in logic")
	}
}

func (b *Bencode) AddBString(bs *BencodeString) {
	b.BString = bs
	b.panicIfMultipleAssignment()
}

func (b *Bencode) AddBInt(bi *BencodeInt) {
	b.BInt = bi
	b.panicIfMultipleAssignment()
}

func (b *Bencode) AddBList(bl *BencodeList) {
	b.BList = bl
	b.panicIfMultipleAssignment()
}

func (b *Bencode) AddBDict(bd *BencodeDict) {
	b.BDict = bd
	b.panicIfMultipleAssignment()
}

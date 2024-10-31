package bencodingParser

import (
	"fmt"
	"log"
	"strconv"
)

func bencodeType(bencode *Bencode) int {
	if bencode == nil {
		return -1
	}

	if bencode.BString != nil {
		return StringType
	} else if bencode.BInt != nil {
		return IntegerType
	} else if bencode.BList != nil {
		return ListType
	} else if bencode.BDict != nil {
		return DictionaryType
	}

	return -1
}

func encodeInt(bencode *Bencode) ([]byte, error) {
	var data = make([]byte, 0)

	if bencode == nil || bencode.BInt == nil {
		return nil, fmt.Errorf("bencode parsing error: 'bencode' struct or its 'BInt' field is nil; please initialize before processing")
	}

	data = append(data, 'i')
	data = append(data, []byte(strconv.FormatInt(int64(*bencode.BInt), 10))...)
	data = append(data, 'e')
	return data, nil
}

func encodeString(bencode *Bencode) ([]byte, error) {
	var data = make([]byte, 0)

	if bencode == nil || bencode.BString == nil {
		return nil, fmt.Errorf("bencode parsing error: 'bencode' struct or its 'BString' field is nil; please initialize before processing")
	}

	strLen := len(string(*bencode.BString))
	data = append(data, []byte(strconv.Itoa(strLen)+":")...)
	data = append(data, []byte(*bencode.BString)...)
	return data, nil
}

func encodeList(bencode *Bencode) ([]byte, error) {
	var data = make([]byte, 0)

	if bencode == nil || bencode.BList == nil {
		return nil, fmt.Errorf("bencode parsing error: 'bencode' struct or its 'BList' field is nil; please initialize before processing")
	}

	data = append(data, 'l')
	for _, bencodeEle := range *bencode.BList {
		encodedBencodeEle, err := serializeBencode(&bencodeEle)
		if err != nil {
			return nil, err
		}
		data = append(data, encodedBencodeEle...)
	}
	data = append(data, 'e')
	return data, nil
}

func encodeDictionary(bencode *Bencode) ([]byte, error) {
	var data = make([]byte, 0)

	if bencode == nil || bencode.BDict == nil {
		return nil, fmt.Errorf("bencode parsing error: 'bencode' struct or its 'BDict' field is nil; please initialize before processing")
	}
	data = append(data, 'd')
	for key, value := range *bencode.BDict {
		encodedKey, err := encodeString(NewBencodeFromBString(&key))
		if err != nil {
			return nil, err
		}
		data = append(data, encodedKey...)

		encodedValue, err := serializeBencode(&value)
		if err != nil {
			return nil, err
		}
		data = append(data, encodedValue...)
	}
	data = append(data, 'e')
	return data, nil
}

func serializeBencode(bencode *Bencode) ([]byte, error) {
	serializedBencodeType := bencodeType(bencode)
	if serializedBencodeType == StringType {
		return encodeString(bencode)
	} else if serializedBencodeType == IntegerType {
		return encodeInt(bencode)
	} else if serializedBencodeType == ListType {
		return encodeList(bencode)
	} else if serializedBencodeType == DictionaryType {
		return encodeDictionary(bencode)
	}

	log.Printf("Error: unsupported Bencode type: %T", bencode)
	return nil, fmt.Errorf("unsupported Bencode type: %T; expected StringType, IntegerType, ListType, or DictionaryType", bencode)
}

func SerializeBencode(bencode *Bencode) ([]byte, error) {
	if bencode == nil {
		return nil, fmt.Errorf("serialized bencode is nil; please initialize before processing")
	}

	data, err := serializeBencode(bencode)
	if err != nil {
		log.Printf("failed to encode the serialized bencode")
		return nil, fmt.Errorf("failed to encode serialized Bencode: %w", err) // Wrap the error with context
	}
	return data, err
}

package main

import (
	bencodingParser "bittorrent-client/bencoding-parser"
	"crypto/sha1"
	"encoding/hex"
	"log"
)

func ComputeInfoHash(infoDictionaryBencode *bencodingParser.Bencode) string {
	dataInfo, err := bencodingParser.SerializeBencode(infoDictionaryBencode)
	if err != nil {
		log.Fatalf("error encoding the info dictionary")
	}

	hasher := sha1.New()
	hasher.Write(dataInfo)

	hash := hasher.Sum(nil)
	hashHex := hex.EncodeToString(hash)
	return hashHex
}

//func main() {
//	fileName := "file2.torrent"
//	file, err := os.Open(fileName)
//	if err != nil {
//		log.Fatalf("error opening file: %v", err)
//	}
//
//	bencode, err := bencodingParser.ParseBencodeTorrentFile(file)
//	if bencode == nil || bencode.BDict == nil {
//		log.Fatalf("bencode is nil")
//	}
//	infoDictionaryBencode, _ := (*bencode.BDict).Get(InfoKey)
//	infoHash := ComputeInfoHash(infoDictionaryBencode)
//	fmt.Println(infoDictionaryBencode)
//	fmt.Println(infoHash)
//}

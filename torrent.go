package main

import (
	bencodingParser "bittorrent-client/bencoding-parser"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

// TorrentType for single or multi file types
type TorrentType int

const (
	SingleFile TorrentType = iota
	MultiFile
)

const (
	AnnounceKey     = "announce"
	AnnounceListKey = "announce-list"
	CommentKey      = "comment"
	CreatedByKey    = "created by"
	CreationDateKey = "creation date"
	EncodingKey     = "encoding"
	UrlListKey      = "url-list"
	InfoKey         = "info"
	NameKey         = "name"
	PieceLengthKey  = "piece length"
	PiecesKey       = "pieces"
	LengthKey       = "length"
	PathKey         = "path"
	FilesKey        = "files"
)

func getTorrentFileType(infoDictionary *bencodingParser.BencodeDict) TorrentType {
	_, exists := infoDictionary.Get(FilesKey)
	if exists {
		return MultiFile
	}
	return SingleFile
}

type Torrent struct {
	Announce      string      // the 'announce' url of the tracker
	AnnounceList  [][]string  // alternate urls for trackers
	CreationDate  time.Time   // creation date of the torrent meta-info file; unix timestamp
	Comment       string      // comment
	CreatedBy     string      // created by
	Encoding      string      // the character encoding used in this file (UTF-8)
	UrlList       []string    // alternate urls for downloading the resource
	StructureType TorrentType // for single or multi file types
	Info          InfoDict    // info dictionary
	InfoHash      [20]byte    // SHA1 hash of the info dictionary
}

type InfoDict struct {
	Name        string // name of the torrent
	PieceLength int64  // size of each piece in bytes
	Pieces      []byte // binary; byte slice
	Length      int64  // for single-file torrent; total size of file in bytes; for a multi-file torrent contains the total size of all files combined
	Files       []File // for multi-file torrent
}

type File struct {
	Length int64    // total size in bytes for the torrent
	Path   []string // the path to the file as a list of strings
}

func NewTorrent() *Torrent {
	return &Torrent{}
}

func (t *Torrent) String() string {
	return fmt.Sprintf(
		"Torrent{\n\tAnnounce: %s,\n\tAnnounceList: %v,\n\tCreationDate: %s,\n\tComment: %s,\n\tCreatedBy: %s,\n\tEncoding: %s,\n\tUrlList: %v,\n\tStructureType: %s,\n\tInfoHash: %s,\n\tInfo: %s\n}\n ",
		t.Announce,
		t.AnnounceList,
		t.CreationDate.Format(time.RFC3339),
		t.Comment,
		t.CreatedBy,
		t.Encoding,
		t.UrlList,
		t.StructureType.String(),
		hex.EncodeToString(t.InfoHash[:]),
		t.Info.String(),
	)
}

func (tt *TorrentType) String() string {
	if (*tt) == SingleFile {
		return "SingleFile"
	}
	return "MultiFile"
}

func (info *InfoDict) String() string {
	filesStr := ""
	for _, file := range info.Files {
		filesStr += file.String() + "\n" // Call the String method for each File
	}

	return fmt.Sprintf(
		"InfoDict{\n\t\tName: %s,\n\t\tPieceLength: %d,\n\t\tPieces: %d bytes,\n\t\tLength: %d,\n\t\tFiles: %v\n\t}",
		info.Name,
		info.PieceLength,
		len(info.Pieces),
		info.Length,
		filesStr,
	)
}

func (f *File) String() string {
	return fmt.Sprintf(
		"\n\t\t\tFile{\n\t\t\t\tLength: %d,\n\t\t\t\tPath: [%s]\n\t\t\t}",
		f.Length,
		strings.Join(f.Path, "/"),
	)
}

// parseAnnounceUrl Mandatory Field
func parseAnnounceUrl(bencodeTorrentDict *bencodingParser.BencodeDict) string {
	announceBencode, exists := bencodeTorrentDict.Get(AnnounceKey)
	if !exists || announceBencode.BString == nil {
		log.Fatalf("no announce URL in torrent file")
	}

	return string(*announceBencode.BString)
}

func parseOptionalAnnounceList(bencodeTorrentDict *bencodingParser.BencodeDict) [][]string {
	announceListBencode, exists := bencodeTorrentDict.Get(AnnounceListKey)
	if !exists || announceListBencode.BList == nil {
		log.Printf("no 'announce-list' in torrent file")
		return nil
	}

	var announceList [][]string
	for _, trackerUrlsGroupBencode := range *announceListBencode.BList {
		var trackerUrls []string
		for _, trackerUrlBencode := range *trackerUrlsGroupBencode.BList {
			trackerUrls = append(trackerUrls, string(*trackerUrlBencode.BString))
		}
		announceList = append(announceList, trackerUrls)
	}
	return announceList
}

// parseOptionalComment Optional field
func parseOptionalComment(bencodeTorrentDict *bencodingParser.BencodeDict) string {
	commentBencode, exists := bencodeTorrentDict.Get(CommentKey)
	if !exists || commentBencode.BString == nil {
		log.Print("no 'comment' found in the torrent file")
		return ""
	}

	return string(*commentBencode.BString)
}

// parseOptionalCreatedBy Optional Field
func parseOptionalCreatedBy(bencodeTorrentDict *bencodingParser.BencodeDict) string {
	createdByBencode, exists := bencodeTorrentDict.Get(CreatedByKey)
	if !exists || createdByBencode.BString == nil {
		log.Printf("no 'created by' found in the torrent file")
		return ""
	}

	return string(*createdByBencode.BString)
}

// parseOptionalCreationDate Optional Field
func parseOptionalCreationDate(bencodeTorrentDict *bencodingParser.BencodeDict) time.Time {
	creationDateBencode, exists := bencodeTorrentDict.Get(CreationDateKey)
	if !exists || creationDateBencode.BInt == nil {
		log.Printf("no 'creation date' found in the torrent file")
		return time.Time{}
	}

	return time.Unix(int64(*creationDateBencode.BInt), 0)
}

// parseOptionalEncoding Optional Field
func parseOptionalEncoding(bencodeTorrentDict *bencodingParser.BencodeDict) string {
	encodingBencode, exists := bencodeTorrentDict.Get(EncodingKey)
	if !exists || encodingBencode.BString == nil {
		log.Printf("no 'encoding' found in the torrent file")
		return ""
	}
	return string(*encodingBencode.BString)
}

// parseOptionalUrlList Optional Field
func parseOptionalUrlList(bencodeTorrentDict *bencodingParser.BencodeDict) []string {
	urlListBencode, exists := bencodeTorrentDict.Get(UrlListKey)
	var urlList []string
	if !exists || urlListBencode.BList == nil {
		log.Printf("no 'url-list' found in the torrent file")
		return nil
	} else {
		for _, bencodeVal := range *urlListBencode.BList {
			urlList = append(urlList, string(*bencodeVal.BString))
		}
	}
	return urlList
}

// parseInfoDictionary Mandatory Field
func parseInfoDictionary(bencodeTorrentDict *bencodingParser.BencodeDict) InfoDict {
	infoDictionaryBencode, exists := bencodeTorrentDict.Get(InfoKey)
	if !exists {
		log.Fatalf("no 'info' dictionary found in torrent file")
	}
	infoDictionary := infoDictionaryBencode.BDict

	infoDict := InfoDict{}
	infoDict.Name = parseNameInInfoDictionary(infoDictionary)
	infoDict.PieceLength = parsePieceLengthInInfoDictionary(infoDictionary)
	infoDict.Pieces = parsePiecesInInfoDictionary(infoDictionary)

	fileStructureType := getTorrentFileType(infoDictionary)
	if fileStructureType == SingleFile {
		infoDict.Length = parseLengthInInfoDictionary(infoDictionary)
	} else {
		infoDict.Length = parseLengthInInfoDictionaryForMultiFileTorrent(infoDictionary)
		infoDict.Files = parseFilesInInfoDictionary(infoDictionary)
	}
	return infoDict
}

// parseNameInInfoDictionary Mandatory field in the info dictionary
func parseNameInInfoDictionary(infoDictionary *bencodingParser.BencodeDict) string {
	name, exists := infoDictionary.Get(NameKey)
	if !exists || name.BString == nil {
		log.Fatalf("no 'name' field found in info dictionary of the torrent file")
	}

	return string(*name.BString)
}

// parsePieceLengthInInfoDictionary Mandatory field in the info dictionary
func parsePieceLengthInInfoDictionary(infoDictionary *bencodingParser.BencodeDict) int64 {
	pieceLength, exists := infoDictionary.Get(PieceLengthKey)
	if !exists || pieceLength == nil {
		log.Fatalf("no 'piece length' field found in info dictionary of the torrent file")
	}

	return int64(*pieceLength.BInt)
}

// parsePiecesInInfoDictionary Mandatory field in the info dictionary
func parsePiecesInInfoDictionary(infoDictionary *bencodingParser.BencodeDict) []byte {
	pieces, exists := infoDictionary.Get(PiecesKey)
	if !exists || pieces.BString == nil {
		log.Fatalf("no 'pieces' field found in info dictionary of the torrent file")
	}

	return []byte(*pieces.BString)
}

// parseLengthInInfoDictionary Mandatory field for a single file torrent
func parseLengthInInfoDictionary(infoDictionary *bencodingParser.BencodeDict) int64 {
	length, exists := infoDictionary.Get(LengthKey)
	if !exists || length.BInt == nil {
		log.Fatalf("no 'length' field found in info dictionary of the torrent file")
	}

	return int64(*length.BInt)
}

func parseLengthInInfoDictionaryForMultiFileTorrent(infoDictionary *bencodingParser.BencodeDict) int64 {
	files, exists := infoDictionary.Get(FilesKey)
	if !exists || files.BList == nil {
		log.Fatalf("no 'files' field found in info dictionary of the torrent file")
	}

	totalLength := int64(0)
	for _, bencodedFile := range *files.BList {
		fileLengthBencode, exists := (*bencodedFile.BDict).Get(LengthKey)
		if !exists || fileLengthBencode.BInt == nil {
			log.Fatalf("no 'length' field found for a file")
		}
		fileLength := int64(*fileLengthBencode.BInt)

		totalLength += fileLength
	}
	return totalLength
}

// parseFilesInInfoDictionary Mandatory field for a multi file torrent
func parseFilesInInfoDictionary(infoDictionary *bencodingParser.BencodeDict) []File {
	files, exists := infoDictionary.Get(FilesKey)
	if !exists || files.BList == nil {
		log.Fatalf("no 'files' field found in info dictionary of the torrent file")
	}

	var filesList []File
	for _, bencodedFile := range *files.BList {
		fileLengthBencode, exists := (*bencodedFile.BDict).Get(LengthKey)
		if !exists || fileLengthBencode.BInt == nil {
			log.Fatalf("no 'length' field found for a file")
		}
		fileLength := int64(*fileLengthBencode.BInt)

		pathBencode, exists := (*bencodedFile.BDict).Get(PathKey)
		if !exists || pathBencode.BList == nil {
			log.Fatalf("no 'path' field found for a file")
		}
		var path []string
		for _, pathSegment := range *pathBencode.BList {
			path = append(path, string(*pathSegment.BString))
		}

		filesList = append(filesList, File{Length: fileLength, Path: path})
	}

	return filesList
}

func ComputeInfoHash(bencodeTorrentDict *bencodingParser.BencodeDict) [20]byte {
	infoDictionaryBencode, exists := bencodeTorrentDict.Get(InfoKey)
	if !exists {
		log.Fatalf("no 'info' dictionary found in torrent file")
	}

	serializedInfo, err := bencodingParser.SerializeBencode(infoDictionaryBencode)
	if err != nil {
		log.Fatalf("error encoding the info dictionary")
	}

	hasher := sha1.New()
	hasher.Write(serializedInfo)

	sum := hasher.Sum(nil)
	if len(sum) != 20 {
		log.Fatalf("SHA-1 hash length mismatch, expected 20 bytes, obtained %d bytes", len(sum))
	}

	var infoHash [20]byte
	copy(infoHash[:], sum)

	return infoHash
}

func LoadTorrent(reader io.Reader) (*Torrent, error) {
	bencode, err := bencodingParser.ParseBencodeTorrentFile(reader)
	if err != nil || bencode.BDict == nil {
		log.Fatalf("error parsing the file: %v\n", err)
	}
	bencodeTorrentDict := bencode.BDict

	torrent := NewTorrent()
	torrent.Announce = parseAnnounceUrl(bencodeTorrentDict)
	torrent.Comment = parseOptionalComment(bencodeTorrentDict)
	torrent.CreatedBy = parseOptionalCreatedBy(bencodeTorrentDict)
	torrent.CreationDate = parseOptionalCreationDate(bencodeTorrentDict)
	torrent.Encoding = parseOptionalEncoding(bencodeTorrentDict)
	torrent.UrlList = parseOptionalUrlList(bencodeTorrentDict)
	torrent.AnnounceList = parseOptionalAnnounceList(bencodeTorrentDict)
	torrent.Info = parseInfoDictionary(bencodeTorrentDict)

	bencodeInfoDictionary, _ := bencodeTorrentDict.Get(InfoKey)
	torrent.StructureType = getTorrentFileType(bencodeInfoDictionary.BDict)

	torrent.InfoHash = ComputeInfoHash(bencodeTorrentDict)
	return torrent, nil
}

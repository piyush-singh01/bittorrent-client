package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type TorrentFileSystem struct {
	baseDir     string
	totalLength int64
	files       []*TorrentFile
	fileOffset  []int64
}
type TorrentFile struct {
	path           []string
	length         int64
	startingOffset int64
}

type TorrentPiece struct {
	index        int
	length       int64
	blocks       []*TorrentBlock
	expectedHash [20]byte
}

type TorrentBlock struct {
	pieceIndex int
	offset     int64  // offset relative to the piece
	block      []byte // TODO: make this a constant size of 16 KB
}

func NewTorrentFileSystemSingleFile(torrent *Torrent) *TorrentFileSystem {
	dirName := strings.TrimSuffix(torrent.Info.Name, filepath.Ext(torrent.Info.Name))
	torrentFile := &TorrentFile{
		path:           []string{dirName, torrent.Info.Name},
		length:         torrent.Info.Length,
		startingOffset: 0,
	}
	fileOffset := []int64{0}

	return &TorrentFileSystem{
		baseDir:     torrent.Info.Name,
		totalLength: torrent.Info.Length,
		files:       []*TorrentFile{torrentFile},
		fileOffset:  fileOffset,
	}
}

func allocateBytesToEmptyFile(file *os.File, sizeInBytes int64) error {
	if sizeInBytes <= 0 {
		return fmt.Errorf("too few bytes to allocate")
	}

	if _, err := file.Seek(sizeInBytes-1, io.SeekStart); err != nil {
		return err
	}
	if _, err := file.Write([]byte{0}); err != nil {
		return err
	}
	return nil
}

func (tfs *TorrentFileSystem) Build() error {
	err := os.MkdirAll(tfs.baseDir, os.ModePerm)
	if err != nil {
		return ErrCreatingDirectory(tfs.baseDir)
	}
	for _, file := range tfs.files {
		// path : []strings
		// length : int64
		// startingOffset : int64
		dirFilePath := filepath.Join(file.path[:len(file.path)-1]...)
		err = os.MkdirAll(dirFilePath, os.ModePerm)
		if err != nil {
			return ErrCreatingDirectory(dirFilePath)
		}

		fileName := file.path[len(file.path)-1]
		osFile, err := os.Create(fileName)
		if err != nil {
			return ErrCreatingFile(fileName)
		}

		err = allocateBytesToEmptyFile(osFile, file.length)
		if err != nil {
			return ErrAllocatingBytes(fileName)
		}

		err = osFile.Close()
		if err != nil {
			return ErrClosingFile(fileName)
		}
	}
	return nil
}

func NewTorrentFile(path []string, length int64, offset int64) *TorrentFile {
	return &TorrentFile{
		path: path,
	}
}

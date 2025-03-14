package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const BlockSize = 1 << 14 // 16KB

/* In the OS, write at piece level, not block level, this will require major modifications lmao*/
/* This is more convenient, since, it will not require frequent opening, closing or file io */
/* Also, we can compare the hash at last with the whole piece */
/* Write to the disk only when the whole piece is obtained, till then keep it in memory */
/* The numBlocksPerPiece global variable does not carry any meaning, since all pieces do not have the same number of blocks, the last one might have less, and the last one might even be smaller*/
/* TODO: Add a read piece and write piece methods, and in read block and write block methods, keep it in the memory till then */
/* TODO: Bruh, but read block might not be present in the memory when it has been written to the disk, keep a method to */
/* TODO: Also, when the last block is written, automatically call the method to write it into a piece */

type TorrentFileSystem struct {
	mu sync.Mutex

	baseDir      string
	totalLength  int64
	files        []*TorrentFile
	fileOffset   []int64
	pieces       []*TorrentPiece
	pieceMutexes []sync.RWMutex

	pieceLength int64
	numPieces   int64

	numBlocksPerPiece int64
}

type TorrentFile struct {
	mu             sync.RWMutex
	osFile         *os.File
	path           []string
	length         int64
	startingOffset int64
}

type TorrentPiece struct {
	index  uint
	length int64

	complete     bool
	hasBlock     []bool
	expectedHash [20]byte
}

func NewTorrentPiece(index uint, length int64, numBlocksPerPiece int64, expectedHash [20]byte) *TorrentPiece {
	return &TorrentPiece{
		index:    index,
		length:   length,
		complete: false,

		hasBlock:     make([]bool, numBlocksPerPiece),
		expectedHash: expectedHash,
	}
}

func NewTorrentFile(path []string, length int64, offset int64) *TorrentFile {
	return &TorrentFile{
		path:           path,
		length:         length,
		startingOffset: offset,
	}
}

func NewTorrentFileSystemSingleFile(torrent *Torrent) *TorrentFileSystem {
	dirName := strings.TrimSuffix(torrent.Info.Name, filepath.Ext(torrent.Info.Name))
	torrentFile := NewTorrentFile([]string{dirName, torrent.Info.Name}, torrent.Info.Length, 0)
	fileOffset := []int64{0, torrent.Info.Length}

	numBlocksPerPiece := assertAndReturnPerfectDivision(torrent.Info.PieceLength, int64(BlockSize))

	pieces := populatePiecesSlice(torrent, numBlocksPerPiece)
	return &TorrentFileSystem{
		baseDir:           dirName,
		totalLength:       torrent.Info.Length,
		files:             []*TorrentFile{torrentFile},
		fileOffset:        fileOffset,
		pieceLength:       torrent.Info.PieceLength,
		pieces:            pieces,
		pieceMutexes:      make([]sync.RWMutex, 0),
		numPieces:         ceilDiv(torrent.Info.Length, torrent.Info.PieceLength),
		numBlocksPerPiece: numBlocksPerPiece,
	}
}

func NewTorrentFileSystemMultiFile(torrent *Torrent) *TorrentFileSystem {
	dirName := strings.TrimSuffix(torrent.Info.Name, filepath.Ext(torrent.Info.Name))
	numBlocksPerPiece := assertAndReturnPerfectDivision(torrent.Info.PieceLength, int64(BlockSize))

	var torrentFiles []*TorrentFile
	currentOffset := int64(0)
	var fileOffset []int64
	for _, file := range torrent.Info.Files {
		// TODO: verify in the bencode parser and torrent loader that the files are in order in which they appear in the bencode

		torrentFile := NewTorrentFile(append([]string{dirName}, file.Path...), file.Length, currentOffset)
		fileOffset = append(fileOffset, currentOffset)
		currentOffset += file.Length
		torrentFiles = append(torrentFiles, torrentFile)
	}
	// append the end offset as well
	fileOffset = append(fileOffset, currentOffset)

	if currentOffset != torrent.Info.Length {
		log.Fatalf("flaw in logic, last absolute offset is not equal to the total torrent length")
	}

	pieces := populatePiecesSlice(torrent, numBlocksPerPiece)
	return &TorrentFileSystem{
		baseDir:           dirName,
		totalLength:       torrent.Info.Length,
		files:             torrentFiles,
		fileOffset:        fileOffset,
		pieceLength:       torrent.Info.PieceLength,
		pieces:            pieces,
		pieceMutexes:      make([]sync.RWMutex, 0),
		numPieces:         ceilDiv(torrent.Info.Length, torrent.Info.PieceLength),
		numBlocksPerPiece: numBlocksPerPiece,
	}
}

func (tfs *TorrentFileSystem) BuildOsFileSystem() error {
	// TODO: Only one thread is supposed to access this, so practically no need for this
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	// Create a directory in the present working directory
	if err := os.MkdirAll(tfs.baseDir, os.ModePerm); err != nil {
		return ErrCreatingDirectory(tfs.baseDir)
	}
	for _, file := range tfs.files {
		dirFilePath := filepath.Join(file.path[:len(file.path)-1]...)
		if err := os.MkdirAll(dirFilePath, os.ModePerm); err != nil {
			return ErrCreatingDirectory(dirFilePath)
		}

		fileName := file.path[len(file.path)-1]
		fullFilePath := filepath.Join(dirFilePath, fileName)
		osFile, err := os.Create(fullFilePath)
		if err != nil {
			return ErrCreatingFile(fullFilePath)
		}

		if err = allocateBytesToEmptyFile(osFile, file.length); err != nil {
			_ = osFile.Close()
			return ErrAllocatingBytes(fileName)
		}

		err = osFile.Close()
		if err != nil {
			return ErrClosingFile(fileName)
		}
	}
	return nil
}

func CreateTorrentFileSystem(torrent *Torrent) (*TorrentFileSystem, error) {
	var torrentFileSystem *TorrentFileSystem
	if torrent.StructureType == SingleFile {
		torrentFileSystem = NewTorrentFileSystemSingleFile(torrent)
	} else if torrent.StructureType == MultiFile {
		torrentFileSystem = NewTorrentFileSystemMultiFile(torrent)
	} else {
		return nil, fmt.Errorf("unsupported torrent file type: can not create torrent file system")
	}

	if err := torrentFileSystem.BuildOsFileSystem(); err != nil {
		return nil, fmt.Errorf("error creating torrent file system: %v", err)
	}
	return torrentFileSystem, nil
}

/* TODO: Build a cache to prevent frequent openings and closing a file when calling read or write */

func (tf *TorrentFile) readOpen() error {
	var osFile *os.File
	var err error
	filePath := filepath.Join(tf.path...)

	if tf.osFile != nil {
		log.Printf("The file is already open: %s", filePath)
		return nil
	}

	if osFile, err = os.Open(filePath); err != nil {
		return ErrOpeningFile(filePath)
	}
	tf.osFile = osFile
	return nil
}

func (tf *TorrentFile) readWriteOpen() error {
	var osFile *os.File
	var err error
	filePath := filepath.Join(tf.path...)

	if tf.osFile != nil {
		log.Printf("The file is already open: %s", filePath)
		return nil
	}

	if osFile, err = os.OpenFile(filePath, os.O_RDWR, 0644); err != nil {
		return ErrOpeningFile(filePath)
	}
	tf.osFile = osFile
	return nil
}

func (tf *TorrentFile) close() {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	CloseReadCloserWithLog(tf.osFile)
	tf.osFile = nil
}

func (tf *TorrentFile) readFileAtOffsetAndLength(offset int64, length int64) ([]byte, error) {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	if err := tf.readOpen(); err != nil {
		return nil, err
	}
	defer tf.close()

	// the offset is relative to the file
	buffer := make([]byte, length)

	n, err := tf.osFile.ReadAt(buffer, offset)
	if err != nil {
		return nil, ErrReadingFile(tf.osFile.Name(), err)
	}

	// if short read
	if int64(n) < length {
		return nil, ErrShortRead(tf.osFile.Name(), err)
	}
	return buffer, nil
}

func (tf *TorrentFile) writeFileAtOffset(offset int64, data []byte) (int, error) {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	if err := tf.readWriteOpen(); err != nil {
		return 0, err
	}
	defer tf.close()

	n, err := tf.osFile.WriteAt(data, offset)
	if err != nil {
		return 0, ErrWritingFile(tf.osFile.Name(), err)
	}

	if n < len(data) {
		return n, ErrShortWrite(tf.osFile.Name(), err)
	}
	return n, nil
}

func (tfs *TorrentFileSystem) ReadBlock(pieceIndex int64, relativeOffset int64, length int64) (int64, []byte, error) {

	// -1 since last value in file offset is the end offset (piece length)
	if pieceIndex < 0 || pieceIndex >= tfs.numPieces {
		return 0, nil, fmt.Errorf("piece index out of range")
	}

	if relativeOffset > tfs.pieceLength {
		return 0, nil, fmt.Errorf("relative offset out of range")
	}

	absoluteOffset := relativeOffset + (pieceIndex * tfs.pieceLength)
	offsetToReadTill := absoluteOffset + length

	if offsetToReadTill > tfs.totalLength {
		return 0, nil, fmt.Errorf("end offset out of range")
	}

	tfs.pieceMutexes[pieceIndex].RLock()
	defer tfs.pieceMutexes[pieceIndex].RUnlock()

	var err error
	var buffer = make([]byte, length)

	lengthRead := int64(0)
	currentAbsoluteOffset := absoluteOffset
	for lengthRead < length {
		nextOffsetIndex := findNextOffsetIndex(tfs.fileOffset, currentAbsoluteOffset)
		if nextOffsetIndex == -1 {
			log.Fatalf("flaw in logic, next offset not found")
		}
		currentFile := tfs.files[nextOffsetIndex-1]
		currentOffsetRelativeToFile := currentAbsoluteOffset - currentFile.startingOffset

		nextFileOffset := tfs.fileOffset[nextOffsetIndex]
		lengthToReadInCurrentFile := min(nextFileOffset, offsetToReadTill) - currentAbsoluteOffset

		var fileData []byte
		fileData, err = currentFile.readFileAtOffsetAndLength(currentOffsetRelativeToFile, lengthToReadInCurrentFile)
		if err != nil {
			return lengthRead, nil, err
		}

		// copy data to buffer
		copy(buffer[lengthRead:], fileData)

		lengthRead += int64(len(fileData))
		currentAbsoluteOffset = min(nextFileOffset, offsetToReadTill)
	}

	return lengthRead, buffer, nil
}

func (tfs *TorrentFileSystem) WriteBlock(pieceIndex int64, relativeOffset int64, block []byte) (int64, error) {

	length := int64(len(block))
	if length != BlockSize {
		return 0, ErrInvalidRequest(nil)
	}

	var err error
	var blockIndex int64
	blockIndex, err = findBlockIndex(relativeOffset)
	if err != nil {
		return 0, ErrInvalidRequest(err)
	} else if tfs.pieces[pieceIndex].hasBlock[blockIndex] {
		return 0, ErrBlockAlreadyExists
	}

	if pieceIndex < 0 || pieceIndex >= tfs.numPieces {
		return 0, fmt.Errorf("piece index out of range")
	}

	if relativeOffset > tfs.pieceLength {
		return 0, fmt.Errorf("relative offset out of range")
	}

	absoluteOffset := relativeOffset + (pieceIndex * tfs.pieceLength)
	offsetToWriteTill := absoluteOffset + length

	if offsetToWriteTill > tfs.totalLength {
		return 0, fmt.Errorf("end offset out of range")
	}

	if tfs.pieces[pieceIndex].complete {
		return 0, ErrPieceAlreadyExists
	}

	tfs.pieceMutexes[pieceIndex].Lock()
	defer tfs.pieceMutexes[pieceIndex].Unlock()

	lengthWritten := int64(0)
	currentAbsoluteOffset := absoluteOffset
	for lengthWritten < length {
		nextOffsetIndex := findNextOffsetIndex(tfs.fileOffset, currentAbsoluteOffset)
		if nextOffsetIndex == -1 {
			log.Fatalf("flaw in logic, next offset not found")
		}

		currentFile := tfs.files[nextOffsetIndex-1]
		currentOffsetRelativeToFile := currentAbsoluteOffset - currentFile.startingOffset

		nextFileOffset := tfs.fileOffset[nextOffsetIndex]
		lengthToWriteInCurrentFile := min(nextFileOffset, offsetToWriteTill) - currentAbsoluteOffset

		var n int
		n, err = currentFile.writeFileAtOffset(currentOffsetRelativeToFile, block[lengthWritten:lengthWritten+lengthToWriteInCurrentFile])
		if err != nil {
			return lengthWritten, err
		}

		lengthWritten += int64(n)
		currentAbsoluteOffset = min(nextFileOffset, offsetToWriteTill)
	}

	tfs.pieces[pieceIndex].hasBlock[blockIndex] = true
	return lengthWritten, nil
}

func (tfs *TorrentFileSystem) CleanUp() {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	// closes all files
	for _, file := range tfs.files {
		file.close()
	}
}

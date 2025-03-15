package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type RequestType int

const (
	Read RequestType = iota
	Write
)

type TorrentFileSystem struct {
	mu sync.Mutex

	baseDir      string // the base directory
	totalLength  int64  // total size of the entire torrent in bytes
	files        []*TorrentFile
	fileOffset   []int64
	pieces       []*TorrentPiece
	pieceMutexes []sync.RWMutex
	hasPiece     []bool // if we have the complete piece at (pieceIndex)
	complete     bool   // if we have the entire torrent

	pieceLength int64
	numPieces   int64

	numPiecesObtained int64
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

	numBlocksInPiece   int64
	numBlocksCompleted int64
	complete           bool
	hasBlock           []bool
	expectedHash       [20]byte
}

func NewTorrentPiece(index uint, length int64, numBlocksInPiece int64, expectedHash [20]byte) *TorrentPiece {
	return &TorrentPiece{
		index:  index,
		length: length,

		numBlocksInPiece:   numBlocksInPiece,
		numBlocksCompleted: 0,
		complete:           false,
		hasBlock:           make([]bool, numBlocksInPiece),
		expectedHash:       expectedHash,
	}
}

func NewTorrentFile(path []string, length int64, offset int64) *TorrentFile {
	return &TorrentFile{
		path:           path,
		length:         length,
		startingOffset: offset,
	}
}

func NewTorrentFileSystemSingleFile(torrent *Torrent, dirName string, pieces []*TorrentPiece) *TorrentFileSystem {
	torrentFile := NewTorrentFile([]string{dirName, torrent.Info.Name}, torrent.Info.Length, 0)
	fileOffset := []int64{0, torrent.Info.Length}

	numPieces := ceilDiv(torrent.Info.Length, torrent.Info.PieceLength)
	return &TorrentFileSystem{
		baseDir:      dirName,
		totalLength:  torrent.Info.Length,
		files:        []*TorrentFile{torrentFile},
		fileOffset:   fileOffset,
		pieceLength:  torrent.Info.PieceLength,
		pieces:       pieces,
		pieceMutexes: make([]sync.RWMutex, numPieces),
		numPieces:    numPieces,

		complete:          false,
		hasPiece:          make([]bool, numPieces),
		numPiecesObtained: 0,
	}
}

func NewTorrentFileSystemMultiFile(torrent *Torrent, dirName string, pieces []*TorrentPiece) *TorrentFileSystem {
	var torrentFiles []*TorrentFile
	currentOffset := int64(0)
	var fileOffset []int64

	for _, file := range torrent.Info.Files {
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

	numPieces := ceilDiv(torrent.Info.Length, torrent.Info.PieceLength)
	return &TorrentFileSystem{
		baseDir:      dirName,
		totalLength:  torrent.Info.Length,
		files:        torrentFiles,
		fileOffset:   fileOffset,
		pieceLength:  torrent.Info.PieceLength,
		pieces:       pieces,
		pieceMutexes: make([]sync.RWMutex, numPieces),
		numPieces:    numPieces,

		complete:          false,
		hasPiece:          make([]bool, numPieces),
		numPiecesObtained: 0,
	}
}

func (tfs *TorrentFileSystem) BuildOsFileSystem() error {
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
	dirName := strings.TrimSuffix(torrent.Info.Name, filepath.Ext(torrent.Info.Name))
	pieces := populatePiecesSlice(torrent)

	var torrentFileSystem *TorrentFileSystem
	if torrent.StructureType == SingleFile {
		torrentFileSystem = NewTorrentFileSystemSingleFile(torrent, dirName, pieces)
	} else if torrent.StructureType == MultiFile {
		torrentFileSystem = NewTorrentFileSystemMultiFile(torrent, dirName, pieces)
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

// validateRequest validates request body for read/write block/piece
func (tfs *TorrentFileSystem) validateRequest(requestType RequestType, pieceIndex int64, relativeOffset int64, length int64) (int64, int64, error) {
	// validation: piece index within bounds
	if pieceIndex < 0 || pieceIndex >= tfs.numPieces {
		return 0, 0, ErrOutOfRange("piece index")
	}

	// validation: has the complete piece for `Read` and does not has the block for `Write`
	blockIndex, err := findBlockIndex(relativeOffset)
	if err != nil {
		return 0, 0, err
	} else {
		if requestType == Read && !tfs.pieces[pieceIndex].complete {
			// if READ and we do not have the complete piece
			return 0, 0, ErrPieceDoesNotExist
		} else if requestType == Write && tfs.pieces[pieceIndex].hasBlock[blockIndex] {
			// if WRITE and we already have the block
			return 0, 0, ErrBlockAlreadyExists
		} else if requestType != Read && requestType != Write {
			// invalid request type
			return 0, 0, errors.New("neither read nor write")
		}
	}

	// validation: length is equal to block size
	if length != findBlockLength(blockIndex, tfs.pieces[pieceIndex].length, tfs.pieces[pieceIndex].numBlocksInPiece) {
		return 0, 0, ErrInvalidBlockLength
	}

	// validation: relative offset does not lie outside the piece
	if relativeOffset > tfs.pieces[pieceIndex].length {
		return 0, 0, ErrOutOfRange("relative offset")
	}

	absoluteOffset := relativeOffset + (pieceIndex * tfs.pieceLength)
	offsetToProcessTill := absoluteOffset + length

	// validation: if absolute end offset does not lie outside the torrent
	if offsetToProcessTill > tfs.totalLength {
		return 0, 0, ErrOutOfRange("end offset")
	}

	return absoluteOffset, offsetToProcessTill, nil
}

func (tp *TorrentPiece) validateCompletePiece(torrentFileSystem *TorrentFileSystem) {
	retries := 3
	for i := 0; i < retries; i++ {
		_, piece, err := torrentFileSystem.ReadPiece(int64(tp.index))
		if err != nil {
			log.Printf("error reading piece: %v", err)
			if i < retries-1 {
				log.Printf("retrying")
			}
			continue
		}
		if !verifySHA1(piece, tp.expectedHash) {
			log.Printf("calculated hash does not match expected hash for piece index: %d\n", tp.index)
			break
		}

		// validate the piece, if everything is validated
		tp.complete = true
		torrentFileSystem.hasPiece[tp.index] = true
		torrentFileSystem.numPiecesObtained++

		// TODO: Broadcast `have`, update bitfield etc.
		if torrentFileSystem.numPieces == torrentFileSystem.numPiecesObtained {
			// TODO: if all pieces are obtained
		}
		return
	}

	log.Printf("piece can not be validated, invalidating the piece")
	tp.invalidatePiece(torrentFileSystem)
	return
}

func (tp *TorrentPiece) invalidatePiece(torrentFileSystem *TorrentFileSystem) {
	tp.complete = false
	for i := range tp.hasBlock {
		tp.hasBlock[i] = false
	}
	tp.numBlocksCompleted = 0
	torrentFileSystem.hasPiece[tp.index] = false
}

func (tfs *TorrentFileSystem) ReadBlock(pieceIndex int64, relativeOffset int64, length int64) (int64, []byte, error) {

	/* REQUEST VALIDATION */
	absoluteOffset, offsetToReadTill, err := tfs.validateRequest(Read, pieceIndex, relativeOffset, length)
	if err != nil {
		return 0, nil, ErrInvalidRequest(err)
	}

	/* LOCK THE PIECE MUTEX */
	tfs.pieceMutexes[pieceIndex].RLock()
	defer tfs.pieceMutexes[pieceIndex].RUnlock()

	/* READ FILE BY FILE */
	return tfs.readFileByFile(length, absoluteOffset, offsetToReadTill)
}

func (tfs *TorrentFileSystem) WriteBlock(pieceIndex int64, relativeOffset int64, block []byte) (int64, error) {

	/* REQUEST VALIDATION */
	length := int64(len(block))
	absoluteOffset, offsetToWriteTill, err := tfs.validateRequest(Write, pieceIndex, relativeOffset, length)
	if err != nil {
		return 0, ErrInvalidRequest(err)
	}

	/* LOCK THE PIECE MUTEX */
	tfs.pieceMutexes[pieceIndex].Lock()
	defer tfs.pieceMutexes[pieceIndex].Unlock()

	/* WRITE FILE BY FILE */
	lengthWritten, err := tfs.writeFileByFile(block, length, absoluteOffset, offsetToWriteTill)
	if err != nil {
		return 0, err
	}

	/* UPDATE FIELDS AFTER WRITING A BLOCK */
	blockIndex, _ := findBlockIndex(relativeOffset)
	tfs.pieces[pieceIndex].hasBlock[blockIndex] = true
	tfs.pieces[pieceIndex].numBlocksCompleted++

	/* IF ALL BLOCKS ARE COMPLETED */
	if tfs.pieces[pieceIndex].numBlocksCompleted == tfs.pieces[pieceIndex].numBlocksInPiece {
		tfs.pieces[pieceIndex].validateCompletePiece(tfs)
	}
	return lengthWritten, nil
}

func (tfs *TorrentFileSystem) ReadPiece(pieceIndex int64) (int64, []byte, error) {

	/* REQUEST VALIDATION */
	if pieceIndex < 0 || pieceIndex >= tfs.numPieces {
		return 0, nil, ErrInvalidRequest(ErrOutOfRange("piece index"))
	}

	if !tfs.pieces[pieceIndex].complete {
		return 0, nil, ErrInvalidRequest(ErrPieceDoesNotExist)
	}

	absoluteOffset := pieceIndex * tfs.pieceLength
	offsetToReadTill := min(absoluteOffset+tfs.pieceLength, tfs.totalLength) // min for the last piece
	lengthToRead := offsetToReadTill - absoluteOffset

	/* LOCK THE PIECE MUTEX */
	tfs.pieceMutexes[pieceIndex].RLock()
	defer tfs.pieceMutexes[pieceIndex].RUnlock()

	/* READ FILE BY FILE */
	return tfs.readFileByFile(lengthToRead, absoluteOffset, offsetToReadTill)
}

func (tfs *TorrentFileSystem) readFileByFile(lengthToRead int64, absoluteOffset int64, offsetToReadTill int64) (int64, []byte, error) {
	var err error
	var buffer = make([]byte, lengthToRead)

	lengthRead := int64(0)
	currentAbsoluteOffset := absoluteOffset
	for lengthRead < lengthToRead {
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

		copy(buffer[lengthRead:], fileData)

		lengthRead += int64(len(fileData))
		currentAbsoluteOffset = min(nextFileOffset, offsetToReadTill)
	}
	return lengthRead, buffer, nil
}

func (tfs *TorrentFileSystem) writeFileByFile(block []byte, lengthToWrite int64, absoluteOffset int64, offsetToWriteTill int64) (int64, error) {
	lengthWritten := int64(0)
	currentAbsoluteOffset := absoluteOffset
	for lengthWritten < lengthToWrite {
		nextOffsetIndex := findNextOffsetIndex(tfs.fileOffset, currentAbsoluteOffset)
		if nextOffsetIndex == -1 {
			log.Fatalf("flaw in logic, next offset not found")
		}

		currentFile := tfs.files[nextOffsetIndex-1]
		currentOffsetRelativeToFile := currentAbsoluteOffset - currentFile.startingOffset

		nextFileOffset := tfs.fileOffset[nextOffsetIndex]
		lengthToWriteInCurrentFile := min(nextFileOffset, offsetToWriteTill) - currentAbsoluteOffset

		n, err := currentFile.writeFileAtOffset(currentOffsetRelativeToFile, block[lengthWritten:lengthWritten+lengthToWriteInCurrentFile])
		if err != nil {
			return lengthWritten, err
		}

		lengthWritten += int64(n)
		currentAbsoluteOffset = min(nextFileOffset, offsetToWriteTill)
	}
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

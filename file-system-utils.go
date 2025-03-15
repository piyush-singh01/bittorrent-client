package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

const BlockSize = 1 << 14 // 16KB

func verifySHA1(data []byte, expectedHash [20]byte) bool {
	computedHash := sha1.Sum(data)
	return computedHash == expectedHash
}

func populatePiecesSlice(torrent *Torrent) []*TorrentPiece {
	pieces := make([]*TorrentPiece, torrent.Info.NumPieces)

	for pieceIndex := range torrent.Info.NumPieces {
		var pieceLength int64
		var numBlocksInPiece int64
		if pieceIndex == torrent.Info.NumPieces-1 {
			/* LAST PIECE : May be smaller than the rest */
			pieceLength = torrent.Info.Length - ((torrent.Info.PieceLength) * int64(torrent.Info.NumPieces-1))
			numBlocksInPiece = ceilDiv(pieceLength, BlockSize)
		} else {
			/* REGULAR PIECE */
			numBlocksInPiece = assertAndReturnPerfectDivision(torrent.Info.PieceLength, int64(BlockSize))
			pieceLength = torrent.Info.PieceLength
		}

		pieces[pieceIndex] = NewTorrentPiece(pieceIndex, pieceLength, numBlocksInPiece, torrent.Info.Pieces[pieceIndex])
	}
	return pieces
}

func findNextOffsetIndex(fileOffset []int64, absoluteOffset int64) int {
	start := 0
	end := len(fileOffset) - 1
	ans := -1
	for start <= end {
		mid := start + (end-start)/2
		if absoluteOffset < fileOffset[mid] {
			ans = mid
			end = mid - 1
		} else {
			start = mid + 1
		}
	}
	return ans
}

func allocateBytesToEmptyFile(file *os.File, sizeInBytes int64) error {
	if file == nil {
		return ErrNullObject("file is nil, can not allocate bytes")
	}

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

func findBlockIndex(relativeOffset int64) (int64, error) {
	// 0, BS, 2BS, 3BS
	if relativeOffset%BlockSize != 0 {
		// TODO: use this error to discard the request / response received from the peer
		return int64(0), ErrOffsetNotDivisibleByBlockSize(relativeOffset, BlockSize)
	}
	return relativeOffset / BlockSize, nil
}

func findBlockLength(blockIndex int64, pieceLength int64, numBlocksInPiece int64) int64 {
	if blockIndex == numBlocksInPiece-1 {
		// if last Block
		return pieceLength - (blockIndex)*BlockSize
	} else {
		return BlockSize
	}
}

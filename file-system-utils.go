package main

import (
	"fmt"
	"io"
	"os"
)

func findNumBlocksLastPiece(info *InfoDict, numBlocksPerPiece int64) int64 {
	expectedTotalLength := int64(info.NumPieces) * info.PieceLength
	sizeOfLastPiece := expectedTotalLength - info.Length // this is the size of last piece
	return ceilDiv(sizeOfLastPiece, BlockSize)
}

func populatePiecesSlice(torrent *Torrent, numBlocksPerPiece int64) []*TorrentPiece {
	pieces := make([]*TorrentPiece, torrent.Info.NumPieces)

	for pieceIndex := range torrent.Info.NumPieces {
		var pieceLength int64
		if pieceIndex == torrent.Info.NumPieces-1 {
			// this is the last piece whose length may be smaller than rest pieces
			pieceLength = torrent.Info.Length - ((torrent.Info.PieceLength) * int64(torrent.Info.NumPieces-1))

		} else {
			pieceLength = torrent.Info.PieceLength
		}
		pieces[pieceIndex] = NewTorrentPiece(pieceIndex, pieceLength, numBlocksPerPiece, torrent.Info.Pieces[pieceIndex])
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

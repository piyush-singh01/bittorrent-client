package main

import (
	"errors"
	"fmt"
)

/* GENERAL */

var ErrInvalidRequest = func(err error) error { return errors.Join(errors.New("invalid request exception"), err) }
var ErrFlawInLogic = func(errMsg string) error { return fmt.Errorf("flaw in logic: %s", errMsg) }
var ErrNullObject = func(errMsg string) error { return fmt.Errorf("null object exception: %s", errMsg) }
var ErrOutOfRange = func(errMsg string) error { return fmt.Errorf("index out of range: %s", errMsg) }

/* KEYS */

var ErrKeyNotPresent = errors.New("key not present in map")
var ErrKeyAlreadyPresent = errors.New("key already present in map")

/* FILES AND DIRECTORIES */

var ErrCreatingDirectory = func(dirName string) error { return fmt.Errorf("error creating directory: %s", dirName) }
var ErrCreatingFile = func(fileName string) error { return fmt.Errorf("error creating file: %s", fileName) }
var ErrOpeningFile = func(fileName string) error { return fmt.Errorf("error opening file: %s", fileName) }
var ErrClosingFile = func(fileName string) error { return fmt.Errorf("error closing file: %s", fileName) }
var ErrAllocatingBytes = func(fileName string) error { return fmt.Errorf("error allocating bytes: %s", fileName) }

/* READ-WRITE*/

var ErrReadingFile = func(fileName string, err error) error {
	return errors.Join(fmt.Errorf("error reading file %s", fileName), err)
}
var ErrShortRead = func(fileName string, err error) error {
	return errors.Join(fmt.Errorf("error reading complete range %s", fileName), err)
}
var ErrWritingFile = func(fileName string, err error) error {
	return errors.Join(fmt.Errorf("error writing file %s", fileName), err)
}
var ErrShortWrite = func(fileName string, err error) error {
	return errors.Join(fmt.Errorf("error writing complete range %s", fileName), err)
}

/* TORRENT FILE SYSTEM */

var ErrBlockAlreadyExists = errors.New("block already exists")
var ErrPieceAlreadyExists = errors.New("piece already exists")

var ErrBlockDoesNotExist = errors.New("block does not exist")
var ErrPieceDoesNotExist = errors.New("piece does not exist")

var ErrInvalidBlockLength = errors.New("invalid block length")
var ErrHashVerificationFailed = errors.New("calculated hash does not match the expected hash")

/* MATH ASSERTIONS */

var ErrOffsetNotDivisibleByBlockSize = func(offset int64, blockSize int64) error {
	return fmt.Errorf("offset : %d, not divisible by block size (%d)", offset, blockSize)
}

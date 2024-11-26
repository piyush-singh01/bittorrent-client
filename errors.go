package main

import "errors"

var ErrKeyNotPresent = errors.New("key not present in map")
var ErrKeyAlreadyPresent = errors.New("key already present in map")

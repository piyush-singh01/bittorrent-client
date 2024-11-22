package main

import (
	"log"
	"time"
)

type RateTracker struct {
	// All maps key are hex encoded peer ID strings
	// All maps are thread safe
	downloadSpeed      *SpeedMap
	downloadedBytes    *BytesMap
	uploadedSpeed      *SpeedMap
	uploadedBytes      *BytesMap
	lastDownloadTime   *TimeMap
	lastUploadTime     *TimeMap
	samplingWindowSize time.Duration
}

func NewRateTracker() *RateTracker {
	return &RateTracker{
		downloadSpeed:      NewSpeedMap(),
		downloadedBytes:    NewBytesMap(),
		lastDownloadTime:   NewTimeMap(),
		uploadedSpeed:      NewSpeedMap(),
		uploadedBytes:      NewBytesMap(),
		lastUploadTime:     NewTimeMap(),
		samplingWindowSize: time.Millisecond * 500,
	}
}

func (rt *RateTracker) GetTotalDownloadSpeed(peerId string) float64 {
	totalSpeed := float64(0)
	rt.downloadSpeed.mp.Range(func(_, peerDownloadSpeed any) bool {
		totalSpeed += peerDownloadSpeed.(float64)
		return true
	})
	return totalSpeed
}

func (rt *RateTracker) RemoveConnection(peerId string) {
	rt.uploadedBytes.Delete(peerId)
	rt.uploadedSpeed.Delete(peerId)
	rt.downloadedBytes.Delete(peerId)
	rt.downloadSpeed.Delete(peerId)
	rt.lastUploadTime.Delete(peerId)
	rt.lastDownloadTime.Delete(peerId)
}

func (rt *RateTracker) RecordDownload(peerId string, bytes int) {
	if _, exists := rt.lastDownloadTime.Get(peerId); !exists {
		rt.downloadedBytes.Put(peerId, 0)
		rt.lastDownloadTime.Put(peerId, time.Now())
	}

	prevDownloadedBytes, _ := rt.downloadedBytes.Get(peerId)
	rt.downloadedBytes.Put(peerId, prevDownloadedBytes+int64(bytes))
	rt.CalculateUploadSpeed(peerId)
}

func (rt *RateTracker) RecordUpload(peerId string, bytes int) {
	if _, exists := rt.lastUploadTime.Get(peerId); !exists {
		rt.uploadedSpeed.Put(peerId, 0)
		rt.lastUploadTime.Put(peerId, time.Now())
	}

	prevUploadedBytes, _ := rt.uploadedBytes.Get(peerId)
	rt.uploadedBytes.Put(peerId, prevUploadedBytes+int64(bytes))
	rt.CalculateUploadSpeed(peerId)
}

func (rt *RateTracker) GetDownloadSpeed(peerId string) float64 {
	//rt.mu.Lock()
	//defer rt.mu.Unlock()

	downloadSpeed, _ := rt.downloadSpeed.Get(peerId)
	return downloadSpeed
}

func (rt *RateTracker) GetUploadSpeed(peerId string) float64 {
	//rt.mu.Lock()
	//defer rt.mu.Unlock()

	uploadSpeed, _ := rt.uploadedSpeed.Get(peerId)
	return uploadSpeed
}

func (rt *RateTracker) CalculateDownloadSpeed(peerId string) {
	peerLastDownloadTime, exists := rt.lastDownloadTime.Get(peerId)
	if !exists {
		log.Fatalf("[fatal] flaw in logic while calculating rate")
	}
	currTime := time.Now()
	duration := currTime.Sub(peerLastDownloadTime)

	if duration > rt.samplingWindowSize {
		bytesDownloadedSinceLastRecord, _ := rt.downloadedBytes.Get(peerId)
		rt.downloadSpeed.Put(peerId, float64(bytesDownloadedSinceLastRecord)/duration.Seconds())

		rt.downloadedBytes.Put(peerId, 0)
		rt.lastDownloadTime.Put(peerId, currTime)
	}
}

func (rt *RateTracker) CalculateUploadSpeed(peerId string) {
	peerLastUploadTime, exists := rt.lastUploadTime.Get(peerId)
	if !exists {
		log.Fatalf("[fatal] flaw in logic while calculating rate")
	}
	currTime := time.Now()
	duration := currTime.Sub(peerLastUploadTime)

	if duration > rt.samplingWindowSize {
		bytesUploadedSinceLastRecord, _ := rt.uploadedBytes.Get(peerId)
		rt.uploadedSpeed.Put(peerId, float64(bytesUploadedSinceLastRecord)/duration.Seconds())

		rt.uploadedBytes.Put(peerId, 0)
		rt.lastUploadTime.Put(peerId, currTime)
	}
}

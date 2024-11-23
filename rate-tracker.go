package main

import (
	"log"
	"sync"
	"time"
)

// RateTracker All operations on this rate tracker are atomic
type RateTracker struct {
	// All maps key are hex encoded peer ID strings
	// All maps are thread safe
	mu                 sync.RWMutex
	downloadSpeed      *SpeedMap
	downloadedBytes    *BytesMap
	uploadSpeed        *SpeedMap
	uploadedBytes      *BytesMap
	lastDownloadTime   *TimeMap
	lastUploadTime     *TimeMap
	totalDownloadSpeed float64
	totalUploadSpeed   float64
	samplingWindowSize time.Duration
	decayFactor        float64
}

func NewRateTracker() *RateTracker {
	return &RateTracker{
		downloadSpeed:      NewSpeedMap(),
		downloadedBytes:    NewBytesMap(),
		lastDownloadTime:   NewTimeMap(),
		uploadSpeed:        NewSpeedMap(),
		uploadedBytes:      NewBytesMap(),
		lastUploadTime:     NewTimeMap(),
		totalDownloadSpeed: float64(0),
		totalUploadSpeed:   float64(0),
		decayFactor:        0.7,
		samplingWindowSize: time.Millisecond * 10,
	}
}

func (rt *RateTracker) StartTotalSpeedCalculator(session *TorrentSession) {
	for range session.rateTrackerTicker.C {
		rt.mu.Lock()

		now := time.Now()
		rt.totalDownloadSpeed = 0
		rt.totalUploadSpeed = 0

		rt.downloadSpeed.Iterate(func(peerId string, speed float64) {
			lastTime, _ := rt.lastDownloadTime.Get(peerId)
			if now.Sub(lastTime) > rt.samplingWindowSize {
				// Decay download speed if inactive
				speed *= rt.decayFactor
				if speed < 1e-3 { // Threshold for removing negligible speeds
					rt.downloadSpeed.Delete(peerId)
				} else {
					rt.downloadSpeed.Put(peerId, speed)
				}
			}
			rt.totalDownloadSpeed += speed
		})

		rt.uploadSpeed.Iterate(func(peerId string, speed float64) {
			lastTime, _ := rt.lastUploadTime.Get(peerId)
			if now.Sub(lastTime) > rt.samplingWindowSize {
				// Decay upload speed if inactive
				speed *= rt.decayFactor
				if speed < 1e-3 { // Threshold for removing negligible speeds
					rt.uploadSpeed.Delete(peerId)
				} else {
					rt.uploadSpeed.Put(peerId, speed)
				}
			}
			rt.totalUploadSpeed += speed
		})

		log.Printf("total upload speed now is %f bytes/sec", rt.totalUploadSpeed)
		log.Printf("total download speed now is %f bytes/sec", rt.totalDownloadSpeed)

		rt.mu.Unlock()
	}
}

func (rt *RateTracker) GetTotalDownloadSpeed() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	return rt.totalDownloadSpeed
}

func (rt *RateTracker) GetTotalUploadSpeed() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	return rt.totalUploadSpeed
}

func (rt *RateTracker) RemoveConnection(peerId string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.uploadedBytes.Delete(peerId)
	rt.uploadSpeed.Delete(peerId)
	rt.downloadedBytes.Delete(peerId)
	rt.downloadSpeed.Delete(peerId)
	rt.lastUploadTime.Delete(peerId)
	rt.lastDownloadTime.Delete(peerId)
}

func (rt *RateTracker) RecordDownload(peerId string, bytes int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if exists := rt.lastDownloadTime.IsPresent(peerId); !exists {
		rt.downloadedBytes.Put(peerId, 0)
		rt.lastDownloadTime.Put(peerId, time.Now())
	}

	prevDownloadedBytes, _ := rt.downloadedBytes.Get(peerId)
	rt.downloadedBytes.Put(peerId, prevDownloadedBytes+int64(bytes))
	rt.CalculateDownloadSpeed(peerId)
}

func (rt *RateTracker) RecordUpload(peerId string, bytes int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if exists := rt.lastUploadTime.IsPresent(peerId); !exists {
		rt.uploadSpeed.Put(peerId, 0)
		rt.lastUploadTime.Put(peerId, time.Now())
	}

	prevUploadedBytes, _ := rt.uploadedBytes.Get(peerId)
	rt.uploadedBytes.Put(peerId, prevUploadedBytes+int64(bytes))
	rt.CalculateUploadSpeed(peerId)
}

func (rt *RateTracker) GetDownloadSpeed(peerId string) float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	downloadSpeed, _ := rt.downloadSpeed.Get(peerId)
	return downloadSpeed
}

func (rt *RateTracker) GetUploadSpeed(peerId string) float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	uploadSpeed, _ := rt.uploadSpeed.Get(peerId)
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
		currDownloadSpeed := float64(bytesDownloadedSinceLastRecord) / duration.Seconds()
		rt.downloadSpeed.Put(peerId, currDownloadSpeed)

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
		currUploadSpeed := float64(bytesUploadedSinceLastRecord) / duration.Seconds()
		rt.uploadSpeed.Put(peerId, currUploadSpeed)

		rt.uploadedBytes.Put(peerId, 0)
		rt.lastUploadTime.Put(peerId, currTime)
	}
}

package main

import (
	"log"
	"sync"
	"time"
)

type RateTrackerConfigurable struct {
	rateTrackerTickerInterval time.Duration

	samplingWindowSize    time.Duration
	decayFactor           float64
	minimumSpeedThreshold float64
}

// RateTracker All operations on this rate tracker are atomic
type RateTracker struct {
	conf *RateTrackerConfigurable

	muDownload sync.RWMutex
	muUpload   sync.RWMutex

	// All maps key are hex encoded peerConnection ID strings
	downloadSpeed   *SpeedMap
	downloadedBytes *BytesMap

	uploadSpeed   *SpeedMap
	uploadedBytes *BytesMap

	lastDownloadTime *TimeMap
	lastUploadTime   *TimeMap

	totalDownloadSpeed float64
	totalUploadSpeed   float64

	rateTrackerTicker *time.Ticker
}

func NewRateTracker() *RateTracker {
	return &RateTracker{
		conf: &RateTrackerConfigurable{
			rateTrackerTickerInterval: time.Second,
			decayFactor:               0.4,
			samplingWindowSize:        time.Millisecond * 10, // todo: increase this and experiment
			minimumSpeedThreshold:     1e-3,
		},

		downloadSpeed:    NewSpeedMap(),
		downloadedBytes:  NewBytesMap(),
		lastDownloadTime: NewTimeMap(),

		uploadSpeed:    NewSpeedMap(),
		uploadedBytes:  NewBytesMap(),
		lastUploadTime: NewTimeMap(),

		totalDownloadSpeed: float64(0),
		totalUploadSpeed:   float64(0),
	}
}

/* RATE TRACKER TICKER */

func (rt *RateTracker) SetRateTrackerTicker() {
	if rt.rateTrackerTicker != nil {
		rt.rateTrackerTicker.Stop()
	}

	rt.rateTrackerTicker = time.NewTicker(rt.conf.rateTrackerTickerInterval)
}

func (rt *RateTracker) StopRateTrackerTicker() {
	if rt.rateTrackerTicker == nil {
		log.Printf("rate tracker ticker is already stopped")
		return
	}
	rt.rateTrackerTicker.Stop()
}

func (rt *RateTracker) StartTotalSpeedCalculator() {
	for range rt.rateTrackerTicker.C {
		rt.muUpload.Lock()
		rt.muDownload.Lock()

		now := time.Now()
		rt.totalDownloadSpeed = 0
		rt.totalUploadSpeed = 0

		rt.downloadSpeed.Iterate(func(peerId string, speed float64) {
			lastTime, _ := rt.lastDownloadTime.Get(peerId)
			if now.Sub(lastTime) > rt.conf.samplingWindowSize {
				// Decay download speed if inactive
				speed *= rt.conf.decayFactor
				if speed < rt.conf.minimumSpeedThreshold { // Threshold for removing negligible speeds
					rt.downloadSpeed.Delete(peerId)
				} else {
					rt.downloadSpeed.Put(peerId, speed)
				}
			}
			rt.totalDownloadSpeed += speed
		})

		rt.uploadSpeed.Iterate(func(peerId string, speed float64) {
			lastTime, _ := rt.lastUploadTime.Get(peerId)
			if now.Sub(lastTime) > rt.conf.samplingWindowSize {
				// Decay upload speed if inactive
				speed *= rt.conf.decayFactor
				if speed < rt.conf.minimumSpeedThreshold { // Threshold for removing negligible speeds
					rt.uploadSpeed.Delete(peerId)
				} else {
					rt.uploadSpeed.Put(peerId, speed)
				}
			}
			rt.totalUploadSpeed += speed
		})

		log.Printf("total upload speed now is %f bytes/sec", rt.totalUploadSpeed)
		log.Printf("total download speed now is %f bytes/sec", rt.totalDownloadSpeed)

		rt.muDownload.Unlock()
		rt.muUpload.Unlock()
	}
}

func (rt *RateTracker) GetTotalDownloadSpeed() float64 {
	rt.muDownload.RLock()
	defer rt.muDownload.RUnlock()

	return rt.totalDownloadSpeed
}

func (rt *RateTracker) GetTotalUploadSpeed() float64 {
	rt.muUpload.RLock()
	defer rt.muUpload.RUnlock()

	return rt.totalUploadSpeed
}

func (rt *RateTracker) RemoveConnection(peerId string) {
	rt.muUpload.Lock()
	rt.muDownload.Lock()
	defer rt.muUpload.Unlock()
	defer rt.muDownload.Unlock()

	rt.uploadedBytes.Delete(peerId)
	rt.uploadSpeed.Delete(peerId)
	rt.downloadedBytes.Delete(peerId)
	rt.downloadSpeed.Delete(peerId)
	rt.lastUploadTime.Delete(peerId)
	rt.lastDownloadTime.Delete(peerId)
}

func (rt *RateTracker) RecordDownload(peerId string, bytes int) {
	rt.muDownload.Lock()
	defer rt.muDownload.Unlock()

	if exists := rt.lastDownloadTime.IsPresent(peerId); !exists {
		rt.downloadedBytes.Put(peerId, 0)
		rt.lastDownloadTime.Put(peerId, time.Now())
	}

	prevDownloadedBytes, _ := rt.downloadedBytes.Get(peerId)
	rt.downloadedBytes.Put(peerId, prevDownloadedBytes+int64(bytes))
	rt.calculateDownloadSpeed(peerId)
}

func (rt *RateTracker) RecordUpload(peerId string, bytes int) {
	rt.muUpload.Lock()
	defer rt.muUpload.Unlock()

	if exists := rt.lastUploadTime.IsPresent(peerId); !exists {
		rt.uploadSpeed.Put(peerId, 0)
		rt.lastUploadTime.Put(peerId, time.Now())
	}

	prevUploadedBytes, _ := rt.uploadedBytes.Get(peerId)
	rt.uploadedBytes.Put(peerId, prevUploadedBytes+int64(bytes))
	rt.calculateUploadSpeed(peerId)
}

func (rt *RateTracker) GetDownloadSpeed(peerId string) float64 {
	rt.muDownload.RLock()
	defer rt.muDownload.RUnlock()

	downloadSpeed, _ := rt.downloadSpeed.Get(peerId)
	return downloadSpeed
}

func (rt *RateTracker) GetUploadSpeed(peerId string) float64 {
	rt.muUpload.RLock()
	defer rt.muUpload.RUnlock()

	uploadSpeed, _ := rt.uploadSpeed.Get(peerId)
	return uploadSpeed
}

func (rt *RateTracker) calculateDownloadSpeed(peerId string) {
	peerLastDownloadTime, exists := rt.lastDownloadTime.Get(peerId)
	if !exists {
		log.Fatalf("[fatal] flaw in logic while calculating rate")
	}
	currTime := time.Now()
	duration := currTime.Sub(peerLastDownloadTime)

	if duration > rt.conf.samplingWindowSize {
		bytesDownloadedSinceLastRecord, _ := rt.downloadedBytes.Get(peerId)
		currDownloadSpeed := float64(bytesDownloadedSinceLastRecord) / duration.Seconds()
		rt.downloadSpeed.Put(peerId, currDownloadSpeed)

		rt.downloadedBytes.Put(peerId, 0)
		rt.lastDownloadTime.Put(peerId, currTime)
	}
}

func (rt *RateTracker) calculateUploadSpeed(peerId string) {
	peerLastUploadTime, exists := rt.lastUploadTime.Get(peerId)
	if !exists {
		log.Fatalf("[fatal] flaw in logic while calculating rate")
	}
	currTime := time.Now()
	duration := currTime.Sub(peerLastUploadTime)

	if duration > rt.conf.samplingWindowSize {
		bytesUploadedSinceLastRecord, _ := rt.uploadedBytes.Get(peerId)
		currUploadSpeed := float64(bytesUploadedSinceLastRecord) / duration.Seconds()
		rt.uploadSpeed.Put(peerId, currUploadSpeed)

		rt.uploadedBytes.Put(peerId, 0)
		rt.lastUploadTime.Put(peerId, currTime)
	}
}

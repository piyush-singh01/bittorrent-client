package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type TrackerClientConfigurable struct {
	responseTimeout    time.Duration
	maxBackoffDuration time.Duration // the timeout till when tracker request is fulfilled, taking in account the exponential backoff
	pollInterval       time.Duration // would get from the response of the tracker
	responseMinPeers   int           // min peers that the tracker should send
}

type TrackerClient struct {
	conf *TrackerClientConfigurable

	announce   string
	httpClient *http.Client

	infoHash          string
	localPeerId       string
	localListenerPort uint16

	lastResponseTime time.Time
	lastResponse     *TrackerResponse

	trackerPollTicker *time.Ticker
}

func NewTrackerClient(torrent *Torrent, session *TorrentSession) *TrackerClient {
	responseTimeout := 10 * time.Second
	return &TrackerClient{
		conf: &TrackerClientConfigurable{
			responseTimeout:    responseTimeout,
			maxBackoffDuration: time.Second * 36,
			responseMinPeers:   4,
		},

		announce: torrent.Announce,
		httpClient: &http.Client{
			Timeout: responseTimeout,
		},

		infoHash:          string(torrent.InfoHash[:]),
		localPeerId:       string(session.localPeerId[:]),
		localListenerPort: session.configurable.listenerPort,
	}
}

type TrackerResponse struct {
	Peers          []Peer
	Interval       uint32
	TrackerId      string
	MinInterval    uint32
	Incomplete     int
	Complete       int
	WarningMessage string
}

func NewEmptyTrackerResponse() *TrackerResponse {
	return &TrackerResponse{}
}

func (tr *TrackerResponse) String() string {
	peerListStr := ""
	for _, peer := range tr.Peers {
		peerListStr += fmt.Sprintln(peer)
	}
	return fmt.Sprintf(
		"TrackerResponse:\n"+
			"- Interval: %d seconds\n"+
			"- Incomplete: %d\n"+
			"- Complete: %d\n"+
			"- Peers Count: %d\n"+
			"- Peers: \n%s",
		tr.Interval, tr.Incomplete, tr.Complete, len(tr.Peers), peerListStr,
	)
}

func (tc *TrackerClient) buildTrackerRequestUrl(uploaded int64, downloaded int64, left int64) (string, error) {
	baseUrl, err := url.Parse(tc.announce)
	if err != nil {
		return "", fmt.Errorf("failed to parse tracker URL: %w", err)
	}
	params := url.Values{
		"info_hash":  []string{tc.infoHash},
		"peer_id":    []string{tc.localPeerId},
		"port":       []string{strconv.Itoa(int(tc.localListenerPort))},
		"uploaded":   []string{strconv.FormatInt(uploaded, 10)},
		"downloaded": []string{strconv.FormatInt(downloaded, 10)},
		"left":       []string{strconv.FormatInt(left, 10)},
		"compact":    []string{"1"},
	}
	baseUrl.RawQuery = params.Encode()
	log.Printf("querying tracker URL: %s", baseUrl.String())
	return baseUrl.String(), nil
}

func (tc *TrackerClient) getTrackerResponse(uploaded int64, downloaded int64, left int64) (*TrackerResponse, error) {
	trackerRequestUrl, err := tc.buildTrackerRequestUrl(uploaded, downloaded, left)
	if err != nil {
		return nil, fmt.Errorf("failed to build tracker request URL: %w", err)
	}

	resp, err := tc.httpClient.Get(trackerRequestUrl)
	if err != nil {
		return nil, fmt.Errorf("failure while sending request to tracker URL: %w", err)
	}

	defer CloseReadCloserWithLog(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	return parseTrackerResponse(body)
}

// GetTrackerResponse queries tracker with exponential backoff
func (tc *TrackerClient) GetTrackerResponse(upload int64, download int64, left int64) (*TrackerResponse, error) {
	backoff := time.Second
	var trackerResponse *TrackerResponse
	var err error
	for {
		trackerResponse, err = tc.getTrackerResponse(upload, download, left)
		if err != nil {
			log.Printf("tracker returned error: %v, retrying...", err)

			time.Sleep(backoff)
			backoff *= 2

			if backoff >= tc.conf.maxBackoffDuration {
				return nil, fmt.Errorf("tracker query timeout")
			}
			continue
		}
		if len(trackerResponse.Peers) >= tc.conf.responseMinPeers {
			break
		}
		if backoff >= tc.conf.maxBackoffDuration {
			return nil, fmt.Errorf("tracker query timeout")
		}
		log.Printf("tracker query failed: only %d peers returned. retrying in %v", len(trackerResponse.Peers), backoff)
		time.Sleep(backoff)
		backoff *= 2
	}
	tc.lastResponse = trackerResponse
	tc.lastResponseTime = time.Now()
	return trackerResponse, nil
}

// TrackerPollHandler Meant to be run as a goroutine
func (tc *TrackerClient) TrackerPollHandler(session *TorrentSession) {
	for {
		if tc.trackerPollTicker == nil {
			log.Printf("tracker poll ticker is nil")
			return
		}

		<-tc.trackerPollTicker.C
		left, downloaded, uploaded := session.state.GetState()
		trackerResponse, err := tc.GetTrackerResponse(uploaded, downloaded, left)
		if err != nil {
			log.Print(err)
			return
		}
		log.Printf("number of peers obtained : %d", len(trackerResponse.Peers))

		tc.SetTrackerPolling()
		tc.HandleTrackerResponse(trackerResponse, session)
	}
}

func (tc *TrackerClient) HandleTrackerResponse(trackerResponse *TrackerResponse, torrentSession *TorrentSession) {
	var wg sync.WaitGroup
	var mutex sync.Mutex
	countSuccessfulHandshakes := 0
	for _, peer := range trackerResponse.Peers {
		wg.Add(1)
		go func(peer Peer) {
			var conn *PeerConnection
			var err error
			defer wg.Done()
			if conn, err = DialPeerWithTimeoutTCP(peer, torrentSession); err != nil {
				log.Print(err)
				return
			}

			if err = PerformHandshake(conn, torrentSession, torrentSession.localPeerId); err != nil {
				log.Printf("error performing handshake with peer %s: %v, closing connection", conn.peerIdStr, err)
				conn.CloseConnection()
				return
			}
			conn.StartReaderAndWriter(torrentSession)

			mutex.Lock()
			countSuccessfulHandshakes++
			mutex.Unlock()

			log.Printf("handshake successful with peer %s", hex.EncodeToString(peer.PeerId[:]))
		}(peer)
	}
	wg.Wait()
	log.Printf("Total number of successful handshakes are: %d\n", countSuccessfulHandshakes)
}

/* TRACKER POLLING TICKER */

func (tc *TrackerClient) SetTrackerPolling() {
	if tc.trackerPollTicker != nil {
		tc.trackerPollTicker.Stop()
	}

	tc.conf.pollInterval = time.Second * time.Duration(tc.lastResponse.Interval)
	tc.trackerPollTicker = time.NewTicker(tc.conf.pollInterval)
}

func (tc *TrackerClient) StopTrackerPolling() {
	if tc.trackerPollTicker == nil {
		log.Printf("tracker polling is already stopped")
	}
	tc.trackerPollTicker.Stop()
}

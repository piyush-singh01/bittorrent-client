package main

import (
	bencodingParser "bittorrent-client/bencoding-parser"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// todo move to utils
func CloseReadCloserWithLog(c io.ReadCloser) {
	if err := c.Close(); err != nil {
		log.Printf("failed to close resource: %v", err)
	}
}

func (t *Torrent) buildTrackerRequestUrl(peerId [20]byte, port uint16) (string, error) {
	baseUrl, err := url.Parse(t.Announce)
	if err != nil {
		return "", fmt.Errorf("failed to parse tracker URL: %w", err)
	}

	params := url.Values{
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerId[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.FormatInt(t.Info.Length, 10)},
		"compact":    []string{"1"},
	}
	baseUrl.RawQuery = params.Encode()
	return baseUrl.String(), nil
}

func getPeerListFromBencode(peerListBencode *bencodingParser.Bencode) ([]Peer, error) {
	var peers []Peer

	// if is compact
	if peerListBencode.BString != nil {
		peerList := []byte(*peerListBencode.BString)
		if len(peerList)%6 != 0 {
			return nil, fmt.Errorf("invalid compact peer list length: expected multiple of 6 bytes, got %d", len(peerList))
		}
		for i := 0; i < len(peerList); i += 6 {
			peerIP := net.IP(peerList[i : i+4])

			peerPort := binary.BigEndian.Uint16(peerList[i+4 : i+6])
			peer := Peer{IP: peerIP, Port: peerPort}
			peers = append(peers, peer)
		}
		return peers, nil
	}

	// if is not compact
	for _, peerBencode := range *peerListBencode.BList {
		if peerBencode.BDict == nil {
			return nil, fmt.Errorf("error parsing peer list: expected BDict but found nil")
		}

		// Port
		peerPortBencode, exists := peerBencode.BDict.Get("port")
		if !exists || peerPortBencode.BInt == nil {
			return nil, fmt.Errorf("invalid peer list recieved. expected key 'port' but not found")
		}

		// IP
		peerIPBencode, exists := peerBencode.BDict.Get("ip")
		if !exists || peerIPBencode.BString == nil {
			return nil, fmt.Errorf("invalid peer list recieved. expected key 'ip' but not found")
		}

		// Peer Id
		peerIdBencode, exists := peerBencode.BDict.Get("peer id")
		if !exists || peerIdBencode.BString == nil {
			return nil, fmt.Errorf("invalid peer list recieved. expected key 'peer id' but not found")
		}

		var peerId [20]byte
		copy(peerId[:], *peerIdBencode.BString)
		peers = append(peers,
			Peer{
				Port:   uint16(*peerPortBencode.BInt),
				IP:     net.ParseIP(string(*peerIPBencode.BString)),
				PeerId: peerId,
			})
	}
	return peers, nil
}

func getPeers(responseBencode *bencodingParser.Bencode) ([]Peer, error) {
	peerListBencode, exists := responseBencode.BDict.Get("peers")
	if !exists {
		return nil, fmt.Errorf("no peer list found in the response")
	}

	return getPeerListFromBencode(peerListBencode)
}

func getInterval(responseBencode *bencodingParser.Bencode) (uint32, error) {
	intervalBencode, exists := responseBencode.BDict.Get("interval")
	if !exists || intervalBencode.BInt == nil {
		return 0, fmt.Errorf("expected key 'interval' but not found in the response")
	}
	return uint32(*intervalBencode.BInt), nil
}

func getComplete(responseBencode *bencodingParser.Bencode) (int, error) {
	completeBencode, exists := responseBencode.BDict.Get("complete")
	if !exists || completeBencode.BInt == nil {
		return 0, fmt.Errorf("expected key 'complete' but not found in the response")
	}
	return int(*completeBencode.BInt), nil
}

func getIncomplete(responseBencode *bencodingParser.Bencode) (int, error) {
	incompleteBencode, exists := responseBencode.BDict.Get("incomplete")
	if !exists || incompleteBencode.BInt == nil {
		return 0, fmt.Errorf("expected key 'incomplete' but not found in the response")
	}
	return int(*incompleteBencode.BInt), nil
}

func (t *Torrent) GetTrackerResponse(peerId [20]byte, port uint16) (*TrackerResponse, error) {
	trackerRequestUrl, err := t.buildTrackerRequestUrl(peerId, port)
	if err != nil {
		return nil, fmt.Errorf("failed to build tracker request URL: %w", err)
	}

	client := http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get(trackerRequestUrl)
	if err != nil {
		return nil, fmt.Errorf("failure while sending request to tracker URL: %w", err)
	}

	defer CloseReadCloserWithLog(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	responseBencode, err := bencodingParser.ParseBencodeFromByteSlice(body)
	if err != nil {
		return nil, fmt.Errorf("error while deserializing response body: %w", err)
	}

	var trackerResponse = NewEmptyTrackerResponse()
	trackerResponse.Peers, err = getPeers(responseBencode)
	trackerResponse.Interval, err = getInterval(responseBencode)
	trackerResponse.Complete, err = getComplete(responseBencode)
	trackerResponse.Incomplete, err = getIncomplete(responseBencode)
	return trackerResponse, err
}

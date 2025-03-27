package main

import (
	bencodingParser "bittorrent-client/bencoding-parser"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

func parseTrackerResponse(respBody []byte) (*TrackerResponse, error) {

	responseBencode, err := bencodingParser.ParseBencodeFromByteSlice(respBody)
	if err != nil {
		return nil, fmt.Errorf("error while deserializing response body: %w", err)
	}

	// only field in case of failure
	if failureMessage, isFailure := checkFailure(responseBencode); isFailure {
		return nil, fmt.Errorf("tracker returned failure with message: %s", failureMessage)
	}

	var trackerResponse = NewEmptyTrackerResponse()

	// mandatory fields
	trackerResponse.Peers, err = getPeers(responseBencode)
	if err != nil {
		return nil, err
	}

	trackerResponse.Interval, err = getInterval(responseBencode)
	if err != nil {
		return nil, err
	}

	// optional fields
	if trackerId, hasTrackerId := getTrackerId(responseBencode); hasTrackerId {
		log.Printf("tracker has the key `tracker id`")
		trackerResponse.TrackerId = trackerId
	}

	if warningMessage, hasWarning := checkWarning(responseBencode); hasWarning {
		log.Printf("[warning] tracker has a warning messsage: %s", warningMessage)
		trackerResponse.WarningMessage = warningMessage
	}

	if minInterval, hasMinInterval := getMinInterval(responseBencode); hasMinInterval {
		log.Printf("tracker has a minimum interval: %d seconds", minInterval)
		trackerResponse.MinInterval = minInterval
	}

	if complete, hasComplete := getComplete(responseBencode); hasComplete {
		log.Printf("tracker has the key `complete`")
		trackerResponse.Complete = complete
	}

	if incomplete, hasIncomplete := getIncomplete(responseBencode); hasIncomplete {
		log.Printf("tracker has the key `incomplete`")
		trackerResponse.Incomplete = incomplete
	}

	return trackerResponse, nil
}

func checkFailure(responseBencode *bencodingParser.Bencode) (string, bool) {
	failureBencode, exists := responseBencode.BDict.Get("failure reason")
	if !exists || failureBencode.BString == nil {
		return "", false
	}
	return string(*failureBencode.BString), true
}

func checkWarning(responseBencode *bencodingParser.Bencode) (string, bool) {
	warningBencode, exists := responseBencode.BDict.Get("warning message")
	if !exists || warningBencode.BString == nil {
		return "", false
	}
	return string(*warningBencode.BString), false
}

func getPeerListFromBencode(peerListBencode *bencodingParser.Bencode) ([]Peer, error) {
	var peers []Peer

	// if is compact
	if peerListBencode.BString != nil {
		generateRemotePeerId := func(peerIP net.IP, peerPort uint16) [20]byte {
			data := fmt.Sprintf("%s:%d", peerIP.String(), peerPort)
			return sha1.Sum([]byte(data))
		}

		peerList := []byte(*peerListBencode.BString)
		if len(peerList)%6 != 0 {
			return nil, fmt.Errorf("invalid compact peer list length: expected multiple of 6 bytes, got %d", len(peerList))
		}
		for i := 0; i < len(peerList); i += 6 {
			peerIP := net.IP(peerList[i : i+4])

			peerPort := binary.BigEndian.Uint16(peerList[i+4 : i+6])
			peer := Peer{IP: peerIP, Port: peerPort, Type: GetIPType(peerIP), PeerId: generateRemotePeerId(peerIP, peerPort)}
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

		peerIP := net.ParseIP(string(*peerIPBencode.BString))

		var peerId [20]byte
		copy(peerId[:], *peerIdBencode.BString)
		peers = append(peers,
			Peer{
				Port:   uint16(*peerPortBencode.BInt),
				IP:     peerIP,
				Type:   GetIPType(peerIP),
				PeerId: peerId,
			})
	}
	return peers, nil
}

func getPeers(responseBencode *bencodingParser.Bencode) ([]Peer, error) {
	peerListBencode, exists := responseBencode.BDict.Get("peers")
	if !exists {
		return nil, fmt.Errorf("expected key 'peers' but not found in the response")
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

func getMinInterval(responseBencode *bencodingParser.Bencode) (uint32, bool) {
	minIntervalBencode, exists := responseBencode.BDict.Get("min interval")
	if !exists || minIntervalBencode.BInt == nil {
		return 0, false
	}
	return uint32(*minIntervalBencode.BInt), true
}

func getComplete(responseBencode *bencodingParser.Bencode) (int, bool) {
	completeBencode, exists := responseBencode.BDict.Get("complete")
	if !exists || completeBencode.BInt == nil {
		return 0, false
	}
	return int(*completeBencode.BInt), true
}

func getIncomplete(responseBencode *bencodingParser.Bencode) (int, bool) {
	incompleteBencode, exists := responseBencode.BDict.Get("incomplete")
	if !exists || incompleteBencode.BInt == nil {
		return 0, false
	}
	return int(*incompleteBencode.BInt), true
}

func getTrackerId(responseBencode *bencodingParser.Bencode) (string, bool) {
	trackerIdBencode, exists := responseBencode.BDict.Get("tracker id")
	if !exists || trackerIdBencode.BString == nil {
		return "", false
	}
	return string(*trackerIdBencode.BString), true
}

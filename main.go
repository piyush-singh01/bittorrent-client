package main

import (
	"log"
	"os"
	"sync"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Open the file
	fileName := "test-torrents/sample.torrent"
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("[fatal] error opening file %s: %v", fileName, err)
	}
	log.Printf("file %s open successful", fileName)

	var wg sync.WaitGroup
	// Loads the file into a 'torrent' object
	torrent, err := LoadTorrent(file)
	if err != nil {
		log.Fatalf("[fatal] error parsing torrent")
	}
	log.Printf("torrent parsed and loaded")
	log.Print("The loaded torrent is : ", torrent)

	// Generates a local peerConnection ID
	localPeerId, err := generateLocalPeerId()
	if err != nil {
		log.Fatalf("[fatal] can not generate local peerConnection id: %v", err)
	}
	log.Printf("local Peer Id generated")

	// Starts a torrent session
	torrentSession, err := NewTorrentSession(torrent, localPeerId)
	if err != nil {
		log.Fatalf("[fatal] can not start a torrent session")
	}
	log.Printf("torrent session created")

	/************************ LISTENER ************************/

	listener, err := CreateAndMountListener(torrentSession)
	if err != nil {
		log.Fatalf("error mounting listener on port %d: %v", torrentSession.configurable.listenerPort, err)
	}
	log.Printf("listener mounted on port %d", torrentSession.configurable.listenerPort)

	// Mounts the TPC listeners, to listen for handshakes
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err = listener.StartListening(torrentSession); err != nil {
			log.Fatalf("error starting listener: %v", err)
		}
	}()
	log.Printf("listener started")
	defer listener.CloseListener()

	/************************ STATE HANDLER ************************/

	state := NewTorrentState(torrent.Info.Length)
	torrentSession.state = state
	wg.Add(1)
	go func() {
		defer wg.Done()
		state.StateHandler()
	}()

	/************************ TRACKER REQUEST/RESPONSE/POLLING ************************/

	trackerClient := NewTrackerClient(torrent, torrentSession)
	torrentSession.trackerClient = trackerClient
	// Explicitly handling first tracker response
	trackerResponse, err := trackerClient.GetTrackerResponse(0, 0, torrent.Info.Length)
	if err != nil {
		log.Fatalf("[fatal] error getting response from tracker: %v", err)
		return
	}
	log.Printf("first tracker response fetched")
	log.Printf("number of peers obtained : %d", len(trackerResponse.Peers))

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("starting tracker response handler")
		trackerClient.HandleTrackerResponse(trackerResponse, torrentSession)
	}()

	trackerClient.SetTrackerPolling()
	log.Printf("tracker poll ticker started")

	// For all peers in the tracker list, perform a handshake
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("starting tracker poll handler")
		trackerClient.TrackerPollHandler(torrentSession)
	}()

	/************************ RATE-TRACKER ************************/

	rateTracker := NewRateTracker()
	torrentSession.rateTracker = rateTracker

	rateTracker.SetRateTrackerTicker()
	log.Printf("rate tracker ticker started")

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("starting speed calculator")
		rateTracker.StartTotalSpeedCalculator()
	}()

	/************************ QUITTER ************************/

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("starting quitter")
		torrentSession.StartQuitter()
	}()

	/************************ KEEP-ALIVE ************************/

	torrentSession.StartKeepAliveTicker()
	log.Printf("keep alive ticker started")
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("starting keep alive handler")
		torrentSession.KeepAliveHandler()
	}()

	/************************ TORRENT-FILE-SYSTEM ************************/

	//_, err = CreateTorrentFileSystem(torrent)
	//if err != nil {
	//	log.Fatalf("[fatal] can not create a torrent file system: %v", err)
	//}
	//log.Printf("created torrent file system")
	log.Printf("torrent file system disabled")

	wg.Wait()
}

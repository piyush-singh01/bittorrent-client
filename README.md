# pTorrent

<p align="center">
  <em>A high-performance BitTorrent client written in Go</em>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#usage">Usage</a> •
  <a href="#performance">Performance</a> •
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="assets/demo.gif" alt="pTorrent Demo" width="600"/>
</p>


*pTorrent* is a fully-featured BitTorrent client that implements the [BitTorrent Protocol Specification v1.0 (BEP 3)](https://www.bittorrent.org/beps/bep_0003.html) with a focus on performance and reliability:

## Installation

### Prerequisites

- Go 1.18 or higher

### Building from Source

```bash
# Clone
git clone https://github.com/piyush-singh01/bittorrent-client
cd bittorrent-client

# Build
go build -o bittorrent-client
```

## Usage

### Basic Usage

```bash
# Download a torrent
./bittorrent-client download path/to/torrent/file.torrent

# Specify download directory
./bittorrent-client download -o /download/directory path/to/torrent/file.torrent

# Limit download speed (in B/s)
./bittorrent-client download --max-download 16384 path/to/torrent/file.torrent
```

### Advanced Options

```bash
# Set maximum number of peers
./bittorrent-client download --max-peers 100 path/to/torrent/file.torrent

# Set port for incoming connections
./bittorrent-client download --port 6881 path/to/torrent/file.torrent

# Enable verbose logging
./bittorrent-client download --verbose path/to/torrent/file.torrent
```

## Features

Designed with a modular architecture that separates concerns and supports efficient concurrent operations, leveraging goroutines and mutexes to handle a highly concurrent environment.

### Key Components

- **Torrent Parser and Loader**: Parses, validates and loads torrent file metadata.
    - Uses a custom Bencode parser for encoding and decoding `.torrent` files.
- **Peer Manager**: Handles peer discovery, connection establishment, concurrent management and closure.
- **Piece Manager**: Implements piece selection algorithm, and finds peers that have the pieces we need.
- **Tracker Client**: Implements a poller which sends requests at specific intervals peer discovery.
- **File System Abstraction**: Implements a virtual file system, which maps pieces and blocks to files and handles disk I/O and integrity checks.
- **Choker**: Implements the choking algorithm.
- **Bitset**: A logical structure for parsing and handling bitfields.
- **Rate Tracker**: Tracks upload/download bandwidth rate.

### Performance

- **Concurrent Downloads**: Utilizes Go's goroutines and mutexes in a highly concurrent environment. Handles **1000+** concurrent connections.
- **Smart Connection Handling**: Prioritizes peer connections based on their download performance.
- **Efficient Piece Selection**: Has an implementation for a custom data structure, which fetches the most rare piece in $O(1)$ time.
- **Resilient Error Handling**: Hash check verification against corrupted data.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

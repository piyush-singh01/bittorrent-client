package main

import (
	"encoding/binary"
	"fmt"
)

/* TOC
PeerMessageType
	Type Definitions

PeerMessage
- Struct Definition
	  BlockRequest
		- struct definition
		- constructor
		- parser
		- serializer (method)
		- String   (method)
	  Piece Response
		// same structure as the block request
	  Cancel Request
		// same structure as the block request

- Constructor (with and without payload)
- Parser
- Serializer

- Message constructor for each message type
*/

type PeerMessageType int8

const (
	KeepAlive     PeerMessageType = -1
	Choke         PeerMessageType = 0
	Unchoke       PeerMessageType = 1
	Interested    PeerMessageType = 2
	NotInterested PeerMessageType = 3
	Have          PeerMessageType = 4
	Bitfield      PeerMessageType = 5
	Request       PeerMessageType = 6
	Piece         PeerMessageType = 7
	Cancel        PeerMessageType = 8
	Extended      PeerMessageType = 84
	//Port // used for dht, not implemented
)

type PeerMessage struct {
	MessageLength uint32
	MessageId     PeerMessageType
	Payload       []byte
}

type BlockRequest struct {
	index  uint32
	begin  uint32
	length uint32
}

func NewBlockRequest(index uint32, begin uint32, length uint32) *BlockRequest {
	return &BlockRequest{
		index:  index,
		begin:  begin,
		length: length,
	}
}

func ParseBlockRequest(message []byte) (*BlockRequest, error) {
	if len(message) < 12 {
		return nil, fmt.Errorf("invalid message length for block request: expected at least 12 bytes, got %d", len(message))
	}

	index := binary.BigEndian.Uint32(message[0:4])
	begin := binary.BigEndian.Uint32(message[4:8])
	length := binary.BigEndian.Uint32(message[8:12])

	return &BlockRequest{
		index:  index,
		begin:  begin,
		length: length,
	}, nil
}

func (b *BlockRequest) Serialize() []byte {
	requestBuf := make([]byte, 12)
	binary.BigEndian.PutUint32(requestBuf[0:4], b.index)
	binary.BigEndian.PutUint32(requestBuf[4:8], b.begin)
	binary.BigEndian.PutUint32(requestBuf[8:12], b.length)
	return requestBuf
}

func (b *BlockRequest) String() string {
	return fmt.Sprintf("BlockRequest{Index: %d, Begin: %d, Length: %d}",
		b.index,
		b.begin,
		b.length,
	)
}

type PieceResponse struct {
	index uint32
	begin uint32
	block []byte
}

func NewPieceResponse(index uint32, begin uint32, block []byte) *PieceResponse {
	return &PieceResponse{
		index: index,
		begin: begin,
		block: block,
	}
}

func ParsePieceResponse(message []byte) (*PieceResponse, error) {
	if len(message) < 12 {
		return nil, fmt.Errorf("invalid message length: expected at least 12 bytes, got %d", len(message))
	}

	index := binary.BigEndian.Uint32(message[0:4])
	begin := binary.BigEndian.Uint32(message[4:8])
	block := message[8:] // the actual piece block

	return &PieceResponse{
		index: index,
		begin: begin,
		block: block,
	}, nil
}

func (p *PieceResponse) Serialize() []byte {
	responseBuf := make([]byte, 12+len(p.block))
	binary.BigEndian.PutUint32(responseBuf[0:4], p.index)
	binary.BigEndian.PutUint32(responseBuf[4:8], p.begin)
	copy(responseBuf[8:], p.block)
	return responseBuf
}

func (p *PieceResponse) String() string {
	return fmt.Sprintf("PieceResponse{Index: %d, Begin: %d, BlockLength: %d}",
		p.index,
		p.begin,
		len(p.block),
	)
}

type CancelRequest struct {
	index  uint32
	begin  uint32
	length uint32
}

func NewCancelRequest(index uint32, begin uint32, length uint32) *CancelRequest {
	return &CancelRequest{
		index:  index,
		begin:  begin,
		length: length,
	}
}

func ParseCancelRequest(data []byte) (*CancelRequest, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("invalid data length: expected at least 12 bytes, got %d", len(data))
	}

	index := binary.BigEndian.Uint32(data[0:4])
	begin := binary.BigEndian.Uint32(data[4:8])
	length := binary.BigEndian.Uint32(data[8:12])

	return &CancelRequest{
		index:  index,
		begin:  begin,
		length: length,
	}, nil
}

func (c *CancelRequest) Serialize() []byte {
	requestBuf := make([]byte, 12)
	binary.BigEndian.PutUint32(requestBuf[0:4], c.index)
	binary.BigEndian.PutUint32(requestBuf[4:8], c.begin)
	binary.BigEndian.PutUint32(requestBuf[8:12], c.length)
	return requestBuf
}

func NewPeerMessage(messageLen uint32, messageId PeerMessageType, payload []byte) *PeerMessage {
	return &PeerMessage{
		MessageLength: messageLen,
		MessageId:     messageId,
		Payload:       payload,
	}
}

func NewPeerMessageNoPayload(messageLen uint32, messageId PeerMessageType) *PeerMessage {
	return &PeerMessage{
		MessageLength: messageLen,
		MessageId:     messageId,
		Payload:       nil,
	}
}

func ParsePeerMessage(data []byte) (*PeerMessage, error) {

	if len(data) < 4 {
		return nil, fmt.Errorf("invalid data length: expected at least 4 bytes, got %d", len(data))
	}

	messageLength := binary.BigEndian.Uint32(data[0:4])

	if messageLength > 0 {
		messageId := int(data[4])

		var payload = make([]byte, messageLength-1)
		copy(payload, data[5:])

		// Validate Message ID
		if (messageId != 84) && (messageId < -1 || messageId > 8) {
			return nil, fmt.Errorf("invalid message id: expected between -1 and 8, or 84, got %d", messageId)
		}

		// Validate payload length
		if messageLength != uint32(len(payload)) {
			return nil, fmt.Errorf("invalid data length: expected %d bytes, got %d", messageLength, len(payload))
		}
		return NewPeerMessage(messageLength, PeerMessageType(messageId), payload), nil
	}

	return NewKeepAliveMessage(), nil
}

func (p *PeerMessage) Serialize() []byte {
	message := make([]byte, p.MessageLength+4)
	binary.BigEndian.AppendUint32(message, p.MessageLength)
	if p.MessageLength > 0 {
		message[4] = byte(p.MessageId)
		copy(message[5:], p.Payload)
	}
	return message
}

func NewKeepAliveMessage() *PeerMessage {
	return NewPeerMessageNoPayload(0, KeepAlive)
}

func NewChokeMessage() *PeerMessage {
	return NewPeerMessageNoPayload(1, Choke)
}

func NewUnchokeMessage() *PeerMessage {
	return NewPeerMessageNoPayload(1, Unchoke)
}

func NewInterestedMessage() *PeerMessage {
	return NewPeerMessageNoPayload(1, Interested)
}

func NewNotInterestedMessage() *PeerMessage {
	return NewPeerMessageNoPayload(1, NotInterested)
}

// NewHaveMessage The payload is the zero-based index of a piece that has just been successfully downloaded and verified via the hash.
func NewHaveMessage(pieceIndex uint32) *PeerMessage {
	var payload = make([]byte, 4)
	binary.BigEndian.PutUint32(payload, pieceIndex)
	return NewPeerMessage(5, Have, payload)
}

func NewBitfieldMessage(bitset *Bitset) *PeerMessage {
	payload := bitset.Serialize()
	return NewPeerMessage(uint32(len(payload)+1), Bitfield, payload)
}

func NewRequestMessage(index uint32, begin uint32, length uint32) *PeerMessage {
	req := NewBlockRequest(index, begin, length)
	payload := req.Serialize()
	return NewPeerMessage(uint32(len(payload)+1), Request, payload)
}

func NewPieceMessage(index uint32, begin uint32, block []byte) *PeerMessage {
	piece := NewPieceResponse(index, begin, block)
	payload := piece.Serialize()
	return NewPeerMessage(uint32(len(payload)+1), Piece, payload)
}

func NewCancelMessage(index uint32, begin uint32, length uint32) *PeerMessage {
	cancelReq := NewCancelRequest(index, begin, length)
	payload := cancelReq.Serialize()
	return NewPeerMessage(uint32(len(payload)+1), Cancel, payload)
}

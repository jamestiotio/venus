package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	net "gx/ipfs/QmPjvxTpVH8qJyQDnxnsxF9kv9jezKD1kozz1hs3fCGsNh/go-libp2p-net"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	host "gx/ipfs/Qmb8T6YBBsjYsVGfrihQLfCJveczZnneSBqBKkYEBWDjge/go-libp2p-host"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	types "github.com/filecoin-project/go-filecoin/types"
)

// HelloProtocol is the libp2p protocol identifier for the hello protocol.
const HelloProtocol = "/fil/hello/1.0.0"

// HelloMsg is the data structure of a single message in the hello protocol.
type HelloMsg struct {
	BestBlockCid    *cid.Cid
	BestBlockHeight uint64
	GenesisHash     *cid.Cid
}

type syncCallback func(from peer.ID, c *cid.Cid, height uint64)

type getBlockFunc func() *types.Block

// Hello implements the 'Hello' protocol handler. Upon connecting to a new
// node, we send them a message containing some information about the state of
// our chain, and receive the same information from them. This is used to
// initiate a chainsync and detect connections to forks.
type Hello struct {
	host host.Host

	genesis *cid.Cid

	// chainSyncCB is called when new peers tell us about their chain
	chainSyncCB syncCallback

	// getBestBlock is used to retrieve the current best block for filling out
	// our hello messages.  TODO this should be updated to use the best tipset
	getBestBlock getBlockFunc
}

// NewHello creates a new instance of the hello protocol and registers it to
// the given host, with the provided callbacks.
func NewHello(h host.Host, gen *cid.Cid, syncCallback syncCallback, getBestBlockFunc getBlockFunc) *Hello {
	hello := &Hello{
		host:         h,
		genesis:      gen,
		chainSyncCB:  syncCallback,
		getBestBlock: getBestBlockFunc,
	}
	h.SetStreamHandler(HelloProtocol, hello.handleNewStream)

	// register for connection notifications
	h.Network().Notify((*helloNotify)(hello))

	return hello
}

func (h *Hello) handleNewStream(s net.Stream) {
	defer s.Close() // nolint: errcheck

	from := s.Conn().RemotePeer()

	var hello HelloMsg
	if err := json.NewDecoder(s).Decode(&hello); err != nil {
		log.Warningf("bad hello message from peer %s: %s", from, err)
		return
	}

	switch err := h.processHelloMessage(from, &hello); err {
	case ErrBadGenesis:
		log.Error("bad genesis, TODO: disconnect from peer")
		return
	default:
		log.Error(err)
	case nil:
		// ok
	}
}

// ErrBadGenesis is the error returned when a missmatch in genesis blocks happens.
var ErrBadGenesis = fmt.Errorf("bad genesis block")

func (h *Hello) processHelloMessage(from peer.ID, msg *HelloMsg) error {
	if !msg.GenesisHash.Equals(h.genesis) {
		return ErrBadGenesis
	}

	h.chainSyncCB(from, msg.BestBlockCid, msg.BestBlockHeight)
	return nil
}

func (h *Hello) getOurHelloMessage() *HelloMsg {
	best := h.getBestBlock()

	return &HelloMsg{
		GenesisHash:     h.genesis,
		BestBlockCid:    best.Cid(),
		BestBlockHeight: uint64(best.Height),
	}
}

func (h *Hello) sayHello(ctx context.Context, p peer.ID) error {
	s, err := h.host.NewStream(ctx, p, HelloProtocol)
	if err != nil {
		return err
	}
	defer s.Close() // nolint: errcheck

	msg := h.getOurHelloMessage()

	return json.NewEncoder(s).Encode(msg)
}

// New peer connection notifications

type helloNotify Hello

func (hn *helloNotify) hello() *Hello {
	return (*Hello)(hn)
}

const helloTimeout = time.Second * 10

func (hn *helloNotify) Connected(n net.Network, c net.Conn) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), helloTimeout)
		defer cancel()
		p := c.RemotePeer()
		if err := hn.hello().sayHello(ctx, p); err != nil {
			log.Warningf("failed to send hello handshake to peer %s: %s", p, err)
		}
	}()
}

func (hn *helloNotify) Listen(n net.Network, a ma.Multiaddr)      {}
func (hn *helloNotify) ListenClose(n net.Network, a ma.Multiaddr) {}
func (hn *helloNotify) Disconnected(n net.Network, c net.Conn)    {}
func (hn *helloNotify) OpenedStream(n net.Network, s net.Stream)  {}
func (hn *helloNotify) ClosedStream(n net.Network, s net.Stream)  {}

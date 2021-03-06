package core

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"

	"github.com/golang/protobuf/ptypes"
	"github.com/textileio/textile-go/pb"
	"github.com/textileio/textile-go/repo"
)

// announce creates an outgoing announce block
func (t *Thread) annouce() (mh.Multihash, error) {
	t.mux.Lock()
	defer t.mux.Unlock()

	msg, err := t.buildAnnounce()
	if err != nil {
		return nil, err
	}

	res, err := t.commitBlock(msg, pb.ThreadBlock_ANNOUNCE, nil)
	if err != nil {
		return nil, err
	}

	if err := t.indexBlock(res, repo.AnnounceBlock, "", ""); err != nil {
		return nil, err
	}

	if err := t.updateHead(res.hash); err != nil {
		return nil, err
	}

	if err := t.post(res, t.Peers()); err != nil {
		return nil, err
	}

	log.Debugf("added ANNOUNCE to %s: %s", t.Id, res.hash.B58String())

	return res.hash, nil
}

// handleAnnounceBlock handles an incoming announce block
func (t *Thread) handleAnnounceBlock(hash mh.Multihash, block *pb.ThreadBlock) (*pb.ThreadAnnounce, error) {
	msg := new(pb.ThreadAnnounce)
	if err := ptypes.UnmarshalAny(block.Payload, msg); err != nil {
		return nil, err
	}

	if err := t.indexBlock(&commitResult{
		hash:   hash,
		header: block.Header,
	}, repo.AnnounceBlock, "", ""); err != nil {
		return nil, err
	}

	// update author info
	pid, err := peer.IDB58Decode(block.Header.Author)
	if err != nil {
		return nil, err
	}
	if err := t.addOrUpdatePeer(pid, block.Header.Address, msg.Username, msg.Inboxes); err != nil {
		return nil, err
	}

	return msg, nil
}

// buildAnnounce builds up a Announce block
func (t *Thread) buildAnnounce() (*pb.ThreadAnnounce, error) {
	msg := &pb.ThreadAnnounce{}
	username, err := t.datastore.Profile().GetUsername()
	if err != nil {
		return nil, err
	}
	if username != nil {
		msg.Username = *username
	}
	for _, ses := range t.datastore.CafeSessions().List() {
		msg.Inboxes = append(msg.Inboxes, ses.CafeId)
	}
	return msg, nil
}

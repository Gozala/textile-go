package repo

import (
	"time"

	"github.com/textileio/textile-go/images"
	"github.com/textileio/textile-go/pb"
)

type Contact struct {
	Id       string    `json:"id"`
	Username string    `json:"username"`
	Inboxes  []string  `json:"inboxes"`
	Added    time.Time `json:"added"`
}

type Thread struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	PrivKey []byte `json:"sk"`
	Head    string `json:"head"`
}

type ThreadPeer struct {
	Id       string `json:"id"`
	ThreadId string `json:"thread_id"`
	Welcomed bool   `json:"welcomed"`
}

type ThreadMessage struct {
	Id       string       `json:"id"`
	PeerId   string       `json:"peer_id"`
	Envelope *pb.Envelope `json:"envelope"`
	Date     time.Time    `json:"date"`
}

type Block struct {
	Id       string    `json:"id"`
	Date     time.Time `json:"date"`
	Parents  []string  `json:"parents"`
	ThreadId string    `json:"thread_id"`
	AuthorId string    `json:"author_id"`
	Type     BlockType `json:"type"`

	DataId       string           `json:"data_id,omitempty"`
	DataKey      []byte           `json:"data_key,omitempty"`
	DataCaption  string           `json:"data_caption,omitempty"`
	DataMetadata *images.Metadata `json:"data_metadata,omitempty"`
}

type DataBlockConfig struct {
	DataId       string           `json:"data_id"`
	DataKey      []byte           `json:"data_key"`
	DataCaption  string           `json:"data_caption"`
	DataMetadata *images.Metadata `json:"data_metadata"`
}

type BlockType int

const (
	MergeBlock BlockType = iota
	IgnoreBlock
	FlagBlock
	JoinBlock
	AnnounceBlock
	LeaveBlock
	FileBlock
	TextBlock
	CommentBlock
	LikeBlock
)

func (b BlockType) Description() string {
	switch b {
	case MergeBlock:
		return "MERGE"
	case IgnoreBlock:
		return "IGNORE"
	case FlagBlock:
		return "IGNORE"
	case JoinBlock:
		return "JOIN"
	case AnnounceBlock:
		return "ANNOUNCE"
	case LeaveBlock:
		return "LEAVE"
	case FileBlock:
		return "FILE"
	case TextBlock:
		return "TEXT"
	case CommentBlock:
		return "COMMENT"
	case LikeBlock:
		return "LIKE"
	default:
		return "INVALID"
	}
}

type Notification struct {
	Id        string           `json:"id"`
	Date      time.Time        `json:"date"`
	ActorId   string           `json:"actor_id"`           // peer id
	Subject   string           `json:"subject"`            // thread name | device name
	SubjectId string           `json:"subject_id"`         // thread id | device id
	BlockId   string           `json:"block_id,omitempty"` // block id
	DataId    string           `json:"data_id,omitempty"`  // photo id, etc.
	Type      NotificationType `json:"type"`
	Body      string           `json:"body"`
	Read      bool             `json:"read"`
}

type NotificationType int

const (
	ReceivedInviteNotification   NotificationType = iota // peerA invited you
	AccountPeerAddedNotification                         // new account peer added
	PeerJoinedNotification                               // peerA joined
	PeerLeftNotification                                 // peerA left
	FileAddedNotification                                // peerA added a photo
	TextAddedNotification                                // peerA added a message
	CommentAddedNotification                             // peerA commented on peerB's photo, video, comment, etc.
	LikeAddedNotification                                // peerA liked peerB's photo, video, comment, etc.
)

func (n NotificationType) Description() string {
	switch n {
	case ReceivedInviteNotification:
		return "RECEIVED_INVITE"
	case AccountPeerAddedNotification:
		return "ACCOUNT_PEER_ADDED"
	case PeerJoinedNotification:
		return "PEER_JOINED"
	case PeerLeftNotification:
		return "PEER_LEFT"
	case FileAddedNotification:
		return "FILE_ADDED"
	case TextAddedNotification:
		return "TEXT_ADDED"
	case CommentAddedNotification:
		return "COMMENT_ADDED"
	case LikeAddedNotification:
		return "LIKE_ADDED"
	default:
		return "INVALID"
	}
}

type CafeSession struct {
	CafeId     string    `json:"cafe_id"`
	Access     string    `json:"access"`
	Refresh    string    `json:"refresh"`
	Expiry     time.Time `json:"expiry"`
	HttpAddr   string    `json:"http_addr"`
	SwarmAddrs []string  `json:"swarm_addrs"`
}

type CafeRequestType int

const (
	CafeStoreRequest CafeRequestType = iota
	CafeStoreThreadRequest
	CafePeerInboxRequest
)

func (rt CafeRequestType) Description() string {
	switch rt {
	case CafeStoreRequest:
		return "STORE"
	case CafeStoreThreadRequest:
		return "STORE_THREAD"
	case CafePeerInboxRequest:
		return "INBOX"
	default:
		return "INVALID"
	}
}

type CafeRequest struct {
	Id       string          `json:"id"`
	PeerId   string          `json:"peer_id"`
	TargetId string          `json:"target_id"`
	CafeId   string          `json:"cafe_id"`
	Type     CafeRequestType `json:"type"`
	Date     time.Time       `json:"date"`
}

type CafeMessage struct {
	Id     string    `json:"id"`
	PeerId string    `json:"peer_id"`
	Date   time.Time `json:"date"`
}

type CafeClientNonce struct {
	Value   string    `json:"value"`
	Address string    `json:"address"`
	Date    time.Time `json:"date"`
}

type CafeClient struct {
	Id       string    `json:"id"`
	Address  string    `json:"address"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"last_seen"`
}

type CafeClientThread struct {
	Id         string `json:"id"`
	ClientId   string `json:"client_id"`
	SkCipher   []byte `json:"sk_cipher"`
	HeadCipher []byte `json:"head_cipher"`
	NameCipher []byte `json:"name_cipher"`
}

type CafeClientMessage struct {
	Id       string    `json:"id"`
	PeerId   string    `json:"peer_id"`
	ClientId string    `json:"client_id"`
	Date     time.Time `json:"date"`
}

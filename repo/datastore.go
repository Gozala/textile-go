package repo

import (
	"database/sql"
	"strings"
	"time"

	"github.com/textileio/textile-go/keypair"
)

type Datastore interface {
	Config() ConfigStore
	Profile() ProfileStore
	Contacts() ContactStore
	Files() FileStore
	Threads() ThreadStore
	ThreadInvites() ThreadInviteStore
	ThreadPeers() ThreadPeerStore
	ThreadMessages() ThreadMessageStore
	Blocks() BlockStore
	Notifications() NotificationStore
	CafeSessions() CafeSessionStore
	CafeRequests() CafeRequestStore
	CafeMessages() CafeMessageStore
	CafeClientNonces() CafeClientNonceStore
	CafeClients() CafeClientStore
	CafeClientThreads() CafeClientThreadStore
	CafeClientMessages() CafeClientMessageStore
	Ping() error
	Close()
}

type Queryable interface {
	BeginTransaction() (*sql.Tx, error)
	PrepareQuery(string) (*sql.Stmt, error)
	PrepareAndExecuteQuery(string, ...interface{}) (*sql.Rows, error)
	ExecuteQuery(string, ...interface{}) (sql.Result, error)
}

type ConfigStore interface {
	Init(pin string) error
	Configure(accnt *keypair.Full, created time.Time) error
	GetAccount() (*keypair.Full, error)
	GetCreationDate() (time.Time, error)
	IsEncrypted() bool
}

type ProfileStore interface {
	SetUsername(username string) error
	GetUsername() (*string, error)
	SetAvatar(uri string) error
	GetAvatar() (*string, error)
}

type ContactStore interface {
	Queryable
	Add(device *Contact) error
	AddOrUpdate(device *Contact) error
	Get(id string) *Contact
	List() []Contact
	ListByAddress(address string) []Contact
	Count() int
	Delete(id string) error
}

type FileStore interface {
	Queryable
	Add(file *File) error
	Get(hash string) *File
	GetByPrimary(mill string, checksum string) *File
	GetBySource(mill string, source string, opts string) *File
	AddTarget(hash string, target string) error
	RemoveTarget(hash string, target string) error
	Count() int
	Delete(hash string) error
}

type ThreadStore interface {
	Queryable
	Add(thread *Thread) error
	Get(id string) *Thread
	GetByKey(key string) *Thread
	List() []Thread
	Count() int
	UpdateHead(id string, head string) error
	Delete(id string) error
}

type ThreadInviteStore interface {
	Queryable
	Add(invite *ThreadInvite) error
	Get(id string) *ThreadInvite
	List() []ThreadInvite
	Delete(id string) error
}

type ThreadPeerStore interface {
	Queryable
	Add(peer *ThreadPeer) error
	List() []ThreadPeer
	ListById(id string) []ThreadPeer
	ListByThread(threadId string) []ThreadPeer
	ListUnwelcomedByThread(threadId string) []ThreadPeer
	WelcomeByThread(thread string) error
	Count(distinct bool) int
	Delete(id string, thread string) error
	DeleteById(id string) error
	DeleteByThread(thread string) error
}

type ThreadMessageStore interface {
	Queryable
	Add(msg *ThreadMessage) error
	List(offset string, limit int) []ThreadMessage
	Delete(id string) error
}

type BlockStore interface {
	Queryable
	Add(block *Block) error
	Get(id string) *Block
	List(offset string, limit int, query string) []Block
	Count(query string) int
	Delete(id string) error
	DeleteByThread(threadId string) error
}

type NotificationStore interface {
	Queryable
	Add(notification *Notification) error
	Get(id string) *Notification
	Read(id string) error
	ReadAll() error
	List(offset string, limit int) []Notification
	CountUnread() int
	Delete(id string) error
	DeleteByActor(actorId string) error
	DeleteBySubject(subjectId string) error
	DeleteByBlock(blockId string) error
}

// Cafe user-side stores

type CafeSessionStore interface {
	AddOrUpdate(session *CafeSession) error
	Get(cafeId string) *CafeSession
	List() []CafeSession
	Delete(cafeId string) error
}

type CafeRequestStore interface {
	Queryable
	Add(req *CafeRequest) error
	List(offset string, limit int) []CafeRequest
	Delete(id string) error
	DeleteByCafe(cafeId string) error
}

type CafeMessageStore interface {
	Queryable
	Add(msg *CafeMessage) error
	List(offset string, limit int) []CafeMessage
	AddAttempt(id string) error
	Delete(id string) error
}

// Cafe host-side stores

type CafeClientNonceStore interface {
	Add(nonce *CafeClientNonce) error
	Get(value string) *CafeClientNonce
	Delete(value string) error
}

type CafeClientStore interface {
	Add(account *CafeClient) error
	Get(id string) *CafeClient
	Count() int
	List() []CafeClient
	ListByAddress(address string) []CafeClient
	UpdateLastSeen(id string, date time.Time) error
	Delete(id string) error
}

type CafeClientThreadStore interface {
	AddOrUpdate(thrd *CafeClientThread) error
	ListByClient(clientId string) []CafeClientThread
	Delete(id string, clientId string) error
	DeleteByClient(clientId string) error
}

type CafeClientMessageStore interface {
	AddOrUpdate(message *CafeClientMessage) error
	ListByClient(clientId string, limit int) []CafeClientMessage
	CountByClient(clientId string) int
	Delete(id string, clientId string) error
	DeleteByClient(clientId string, limit int) error
}

func ConflictError(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

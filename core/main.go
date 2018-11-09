package core

import (
	"context"
	"errors"
	"fmt"
	logger "gx/ipfs/QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV/go-logging"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	utilmain "gx/ipfs/QmebqVUQQqQFhg74FtQFszUJo22Vpr3e8qBAkvvV4ho9HH/go-ipfs/cmd/ipfs/util"
	oldcmds "gx/ipfs/QmebqVUQQqQFhg74FtQFszUJo22Vpr3e8qBAkvvV4ho9HH/go-ipfs/commands"
	"gx/ipfs/QmebqVUQQqQFhg74FtQFszUJo22Vpr3e8qBAkvvV4ho9HH/go-ipfs/core"
	ipfsconfig "gx/ipfs/QmebqVUQQqQFhg74FtQFszUJo22Vpr3e8qBAkvvV4ho9HH/go-ipfs/repo/config"
	"gx/ipfs/QmebqVUQQqQFhg74FtQFszUJo22Vpr3e8qBAkvvV4ho9HH/go-ipfs/repo/fsrepo"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/textileio/textile-go/archive"
	"github.com/textileio/textile-go/ipfs"
	"github.com/textileio/textile-go/keypair"
	"github.com/textileio/textile-go/repo"
	"github.com/textileio/textile-go/repo/config"
	"github.com/textileio/textile-go/repo/db"
	"github.com/textileio/textile-go/service"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log = logging.Logger("tex-node")

// Version is the core version identifier
const Version = "1.0.0"

// Node is the single Textile instance used by mobile and the cmd tool
var Node *Textile

// kQueueFlushFreq how often to flush the message queues
const kQueueFlushFreq = time.Minute * 10

// kMobileQueueFlushFreq how often to flush the message queues on mobile
const kMobileQueueFlush = time.Minute * 1

// Update is used to notify UI listeners of changes
type Update struct {
	Id   string     `json:"id"`
	Name string     `json:"name"`
	Type UpdateType `json:"type"`
}

// UpdateType indicates a type of node update
type UpdateType int

const (
	// ThreadAdded is emitted when a thread is added
	ThreadAdded UpdateType = iota
	// ThreadRemoved is emitted when a thread is removed
	ThreadRemoved
	// AccountPeerAdded is emitted when an account peer (device) is added
	AccountPeerAdded
	// AccountPeerRemoved is emitted when an account peer (device) is removed
	AccountPeerRemoved
)

// AddDataResult wraps added data content id and key
type AddDataResult struct {
	Id      string           `json:"id"`
	Key     string           `json:"key"`
	Archive *archive.Archive `json:"archive,omitempty"`
}

// InitConfig is used to setup a textile node
type InitConfig struct {
	Account     *keypair.Full
	PinCode     string
	RepoPath    string
	SwarmPorts  string
	ApiAddr     string
	CafeApiAddr string
	GatewayAddr string
	IsMobile    bool
	IsServer    bool
	LogLevel    logger.Level
	LogToDisk   bool
	CafeOpen    bool
}

// MigrateConfig is used to define options during a major migration
type MigrateConfig struct {
	PinCode  string
	RepoPath string
}

// RunConfig is used to define run options for a textile node
type RunConfig struct {
	PinCode  string
	RepoPath string
}

// Textile is the main Textile node structure
type Textile struct {
	context        oldcmds.Context
	repoPath       string
	config         *config.Config
	cancel         context.CancelFunc
	ipfs           *core.IpfsNode
	datastore      repo.Datastore
	started        bool
	threads        []*Thread
	online         chan struct{}
	done           chan struct{}
	updates        chan Update
	threadUpdates  chan ThreadUpdate
	notifications  chan repo.Notification
	threadsService *ThreadsService
	threadsOutbox  *ThreadsOutbox
	cafeService    *CafeService
	cafeOutbox     *CafeOutbox
	cafeInbox      *CafeInbox
	mux            sync.Mutex
	writer         io.Writer
}

// common errors
var ErrAccountRequired = errors.New("account required")
var ErrStarted = errors.New("node is started")
var ErrStopped = errors.New("node is stopped")
var ErrOffline = errors.New("node is offline")
var ErrThreadLoaded = errors.New("thread is loaded")

// InitRepo initializes a new node repo
func InitRepo(conf InitConfig) error {
	// ensure init has not been run
	if fsrepo.IsInitialized(conf.RepoPath) {
		return repo.ErrRepoExists
	}

	// check account
	if conf.Account == nil {
		return ErrAccountRequired
	}

	// log handling
	setupLogging(conf.RepoPath, conf.LogLevel, conf.LogToDisk)

	// init repo
	if err := repo.Init(conf.RepoPath, Version); err != nil {
		return err
	}

	// open the repo
	rep, err := fsrepo.Open(conf.RepoPath)
	if err != nil {
		log.Errorf("error opening repo: %s", err)
		return err
	}
	defer rep.Close()

	// if a specific swarm port was selected, set it in the config
	if err := applySwarmPortConfigOption(rep, conf.SwarmPorts); err != nil {
		return err
	}

	// if this is a server node, apply the ipfs server profile
	if err := applyServerConfigOption(rep, conf.IsServer); err != nil {
		return err
	}

	// add account key to ipfs keystore for resolving ipns profile
	sk, err := conf.Account.LibP2PPrivKey()
	if err != nil {
		return err
	}
	if err := rep.Keystore().Put("account", sk); err != nil {
		return err
	}

	// init sqlite datastore
	sqliteDb, err := db.Create(conf.RepoPath, conf.PinCode)
	if err != nil {
		return err
	}
	if err := sqliteDb.Config().Init(conf.PinCode); err != nil {
		return err
	}
	if err := sqliteDb.Config().Configure(conf.Account, time.Now()); err != nil {
		return err
	}

	// handle textile config
	return applyTextileConfigOptions(conf)
}

// MigrateRepo runs _all_ repo migrations, including major
func MigrateRepo(conf MigrateConfig) error {
	// ensure init has been run
	if !fsrepo.IsInitialized(conf.RepoPath) {
		return repo.ErrRepoDoesNotExist
	}

	// force open the repo and datastore (fixme)
	removeLocks(conf.RepoPath)

	// run _all_ repo migrations if needed
	return repo.MigrateUp(conf.RepoPath, conf.PinCode, false)
}

// NewTextile runs a node out of an initialized repo
func NewTextile(conf RunConfig) (*Textile, error) {
	// ensure init has been run
	if !fsrepo.IsInitialized(conf.RepoPath) {
		return nil, repo.ErrRepoDoesNotExist
	}

	// check if repo needs a major migration
	if err := repo.Stat(conf.RepoPath); err != nil {
		return nil, err
	}

	// force open the repo and datastore (fixme)
	removeLocks(conf.RepoPath)

	// build the node
	node := &Textile{repoPath: conf.RepoPath}

	// load textile config
	var err error
	node.config, err = config.Read(conf.RepoPath)
	if err != nil {
		return nil, err
	}

	// log handling
	llevel, err := logger.LogLevel(strings.ToUpper(node.config.Logs.LogLevel))
	if err != nil {
		llevel = logger.ERROR
	}
	node.writer = setupLogging(conf.RepoPath, llevel, node.config.Logs.LogToDisk)

	// run all minor repo migrations if needed
	if err := repo.MigrateUp(conf.RepoPath, conf.PinCode, false); err != nil {
		return nil, err
	}

	// get database handle
	sqliteDb, err := db.Create(conf.RepoPath, conf.PinCode)
	if err != nil {
		return nil, err
	}
	node.datastore = sqliteDb

	// all done
	return node, nil
}

// Start
func (t *Textile) Start() error {
	t.mux.Lock()
	defer t.mux.Unlock()
	if t.started {
		return ErrStarted
	}
	defer func() {
		t.done = make(chan struct{})
		t.started = true

		// log peer and account info
		addr, err := t.Address()
		if err != nil {
			log.Error(err.Error())
			return
		}
		log.Info("node is started")
		log.Infof("peer id: %s", t.ipfs.Identity.Pretty())
		log.Infof("account address: %s", addr)
	}()
	log.Info("starting node...")

	// raise file descriptor limit
	if err := utilmain.ManageFdLimit(); err != nil {
		log.Errorf("setting file descriptor limit: %s", err)
	}

	// check db
	if err := t.touchDatastore(); err != nil {
		return err
	}

	// load account
	accnt, err := t.Account()
	if err != nil {
		return err
	}

	// load swarm ports
	sports, err := loadSwarmPorts(t.repoPath)
	if err != nil {
		return err
	}
	if sports == nil {
		return errors.New("failed to load swarm ports")
	}

	// build update channels
	t.online = make(chan struct{})
	t.updates = make(chan Update, 10)
	t.threadUpdates = make(chan ThreadUpdate, 10)
	t.notifications = make(chan repo.Notification, 10)

	// build queues
	t.cafeInbox = NewCafeInbox(
		func() *CafeService {
			return t.cafeService
		},
		func() *ThreadsService {
			return t.threadsService
		},
		func() *core.IpfsNode {
			return t.ipfs
		},
		t.datastore,
	)
	t.cafeOutbox = NewCafeOutbox(
		func() *CafeService {
			return t.cafeService
		},
		func() *core.IpfsNode {
			return t.ipfs
		},
		t.datastore,
	)
	t.threadsOutbox = NewThreadsOutbox(
		func() *ThreadsService {
			return t.threadsService
		},
		func() *core.IpfsNode {
			return t.ipfs
		},
		t.datastore,
		t.cafeOutbox,
	)

	// start the ipfs node
	log.Debug("creating an ipfs node...")
	if err := t.createIPFS(false); err != nil {
		log.Errorf("error creating offline ipfs node: %s", err)
		return err
	}
	go func() {
		defer close(t.online)
		if err := t.createIPFS(true); err != nil {
			log.Errorf("error creating online ipfs node: %s", err)
			return
		}

		// setup thread service
		t.threadsService = NewThreadsService(
			accnt,
			t.ipfs,
			t.datastore,
			t.Thread,
			t.sendNotification,
		)

		// setup cafe service
		t.cafeService = NewCafeService(accnt, t.ipfs, t.datastore, t.cafeInbox)
		t.cafeService.setAddrs(t.config.Addresses.CafeAPI, *sports)
		if t.config.Cafe.Open {
			t.cafeService.open = true
			t.startCafeApi(t.config.Addresses.CafeAPI)
		}

		// run queues
		go t.runQueues()

		// print swarm addresses
		if err := ipfs.PrintSwarmAddrs(t.ipfs); err != nil {
			log.Errorf(err.Error())
		}
		log.Info("node is online")
	}()

	// setup threads
	for _, mod := range t.datastore.Threads().List() {
		_, err := t.loadThread(&mod)
		if err == ErrThreadLoaded {
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Stop the node
func (t *Textile) Stop() error {
	t.mux.Lock()
	defer t.mux.Unlock()
	if !t.started {
		return ErrStopped
	}
	defer func() {
		t.started = false
		close(t.done)
	}()
	log.Info("stopping node...")

	// close apis
	if err := t.stopCafeApi(); err != nil {
		return err
	}

	// close ipfs node
	t.context.Close()
	t.cancel()
	if err := t.ipfs.Close(); err != nil {
		log.Errorf("error closing ipfs node: %s", err)
		return err
	}

	// close db connection
	t.datastore.Close()
	dsLockFile := filepath.Join(t.repoPath, "datastore", "LOCK")
	os.Remove(dsLockFile)

	// cleanup
	t.threads = nil

	// close update channels
	close(t.updates)
	close(t.threadUpdates)
	close(t.notifications)

	log.Info("node is stopped")

	return nil
}

// Started returns whether or not node is started
func (t *Textile) Started() bool {
	return t.started
}

// Online returns whether or not node is online
func (t *Textile) Online() bool {
	if t.ipfs == nil {
		return false
	}
	return t.started && t.ipfs.OnlineMode()
}

// Mobile returns whether or not node is configured for a mobile device
func (t *Textile) Mobile() bool {
	return t.config.IsMobile
}

// Writer returns the output writer (logger / stdout)
func (t *Textile) Writer() io.Writer {
	return t.writer
}

// Ipfs returns the underlying ipfs node
func (t *Textile) Ipfs() *core.IpfsNode {
	return t.ipfs
}

// OnlineCh returns the online channel
func (t *Textile) OnlineCh() <-chan struct{} {
	return t.online
}

// DoneCh returns the core node done channel
func (t *Textile) DoneCh() <-chan struct{} {
	return t.done
}

// Ping pings another peer
func (t *Textile) Ping(pid peer.ID) (service.PeerStatus, error) {
	if !t.Online() {
		return "", ErrOffline
	}
	return t.cafeService.Ping(pid)
}

// UpdateCh returns the node update channel
func (t *Textile) UpdateCh() <-chan Update {
	return t.updates
}

// ThreadUpdateCh returns the thread update channel
func (t *Textile) ThreadUpdateCh() <-chan ThreadUpdate {
	return t.threadUpdates
}

// NotificationsCh returns the notifications channel
func (t *Textile) NotificationCh() <-chan repo.Notification {
	return t.notifications
}

// PeerId returns peer id
func (t *Textile) PeerId() (peer.ID, error) {
	if !t.started {
		return "", ErrStopped
	}
	return t.ipfs.Identity, nil
}

// RepoPath returns the node's repo path
func (t *Textile) RepoPath() string {
	return t.repoPath
}

// DataAtPath returns raw data behind an ipfs path
func (t *Textile) DataAtPath(path string) ([]byte, error) {
	if !t.started {
		return nil, ErrStopped
	}
	return ipfs.DataAtPath(t.ipfs, path)
}

// LinksAtPath returns ipld links behind an ipfs path
func (t *Textile) LinksAtPath(path string) ([]*ipld.Link, error) {
	if !t.started {
		return nil, ErrStopped
	}
	return ipfs.LinksAtPath(t.ipfs, path)
}

// createIPFS creates an IPFS node
func (t *Textile) createIPFS(online bool) error {
	// open repo
	rep, err := fsrepo.Open(t.repoPath)
	if err != nil {
		log.Errorf("error opening repo: %s", err)
		return err
	}

	// determine routing
	routing := core.DHTOption
	if t.Mobile() {
		routing = core.DHTClientOption
	}

	// assemble node config
	cfg := &core.BuildCfg{
		Repo:      rep,
		Permanent: true, // temporary way to signify that node is permanent
		Online:    online,
		ExtraOpts: map[string]bool{
			"pubsub": true,
			"ipnsps": true,
			"mplex":  true,
		},
		Routing: routing,
	}

	// create the node
	cctx, cancel := context.WithCancel(context.Background())
	nd, err := core.NewNode(cctx, cfg)
	if err != nil {
		return err
	}
	nd.SetLocal(!online)

	// build the context
	ctx := oldcmds.Context{}
	ctx.Online = online
	ctx.ConfigRoot = t.repoPath
	ctx.LoadConfig = func(path string) (*ipfsconfig.Config, error) {
		return fsrepo.ConfigAt(t.repoPath)
	}
	ctx.ConstructNode = func() (*core.IpfsNode, error) {
		return nd, nil
	}

	// attach to textile node
	if t.cancel != nil {
		t.cancel()
	}
	if t.ipfs != nil {
		if err := t.ipfs.Close(); err != nil {
			log.Errorf("error closing prev ipfs node: %s", err)
			return err
		}
	}
	t.context = ctx
	t.cancel = cancel
	t.ipfs = nd

	return nil
}

// runQueues runs each message queue
func (t *Textile) runQueues() {
	var freq time.Duration
	if t.Mobile() {
		freq = kMobileQueueFlush
	} else {
		freq = kQueueFlushFreq
	}
	tick := time.NewTicker(freq)
	defer tick.Stop()
	t.flushQueues()
	for {
		select {
		case <-tick.C:
			t.flushQueues()
		case <-t.done:
			return
		}
	}
}

// flushQueues flushes each message queue
func (t *Textile) flushQueues() {
	if err := t.touchDatastore(); err != nil {
		log.Error(err)
		return
	}
	go t.threadsOutbox.Flush()
	go t.cafeOutbox.Flush()
	go t.cafeInbox.CheckMessages()
}

// threadByBlock returns the thread owning the given block
func (t *Textile) threadByBlock(block *repo.Block) (*Thread, error) {
	if block == nil {
		return nil, errors.New("block is empty")
	}
	var thrd *Thread
	for _, t := range t.threads {
		if t.Id == block.ThreadId {
			thrd = t
			break
		}
	}
	if thrd == nil {
		return nil, errors.New(fmt.Sprintf("could not find thread: %s", block.ThreadId))
	}
	return thrd, nil
}

// loadThread loads a thread into memory from the given on-disk model
func (t *Textile) loadThread(mod *repo.Thread) (*Thread, error) {
	if loaded := t.Thread(mod.Id); loaded != nil {
		return nil, ErrThreadLoaded
	}
	threadConfig := &ThreadConfig{
		RepoPath: t.repoPath,
		Node: func() *core.IpfsNode {
			return t.ipfs
		},
		Datastore: t.datastore,
		Service: func() *ThreadsService {
			return t.threadsService
		},
		ThreadsOutbox: t.threadsOutbox,
		CafeOutbox:    t.cafeOutbox,
		SendUpdate:    t.sendThreadUpdate,
	}
	thrd, err := NewThread(mod, threadConfig)
	if err != nil {
		return nil, err
	}
	t.threads = append(t.threads, thrd)
	return thrd, nil
}

// sendUpdate adds an update to the update channel
func (t *Textile) sendUpdate(update Update) {
	t.updates <- update
}

// sendThreadUpdate adds a thread update to the update channel
func (t *Textile) sendThreadUpdate(update ThreadUpdate) {
	t.threadUpdates <- update
}

// sendNotification adds a notification to the notification channel
func (t *Textile) sendNotification(notification *repo.Notification) error {
	// add to db
	if err := t.datastore.Notifications().Add(notification); err != nil {
		return err
	}

	// broadcast
	t.notifications <- *notification
	return nil
}

// touchDatastore ensures that we have a good db connection
func (t *Textile) touchDatastore() error {
	if err := t.datastore.Ping(); err != nil {
		log.Debug("re-opening datastore...")
		sqliteDB, err := db.Create(t.repoPath, "")
		if err != nil {
			log.Errorf("error re-opening datastore: %s", err)
			return err
		}
		t.datastore = sqliteDB
	}
	return nil
}

// setupLogging hijacks the ipfs logging system, putting output to files
func setupLogging(repoPath string, level logger.Level, files bool) io.Writer {
	var writer io.Writer
	if files {
		writer = &lumberjack.Logger{
			Filename:   path.Join(repoPath, "logs", "textile.log"),
			MaxSize:    10, // megabytes
			MaxBackups: 3,
			MaxAge:     30, // days
		}
	} else {
		writer = os.Stdout
	}
	backendFile := logger.NewLogBackend(writer, "", 0)
	logger.SetBackend(backendFile)
	logging.SetAllLoggers(level)
	return writer
}

// removeLocks force deletes the IPFS repo and SQLite DB lock files
func removeLocks(repoPath string) {
	repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
	os.Remove(repoLockFile)
	dsLockFile := filepath.Join(repoPath, "datastore", "LOCK")
	os.Remove(dsLockFile)
}

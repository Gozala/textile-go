package core

import (
	"errors"
	"fmt"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"

	"github.com/textileio/textile-go/repo"
)

// Notifications lists notifications
func (t *Textile) Notifications(offset string, limit int) []repo.Notification {
	return t.datastore.Notifications().List(offset, limit)
}

// CountUnreadNotifications counts unread notifications
func (t *Textile) CountUnreadNotifications() int {
	return t.datastore.Notifications().CountUnread()
}

// ReadNotification marks a notification as read
func (t *Textile) ReadNotification(id string) error {
	return t.datastore.Notifications().Read(id)
}

// ReadAllNotifications marks all notification as read
func (t *Textile) ReadAllNotifications() error {
	return t.datastore.Notifications().ReadAll()
}

// AcceptThreadInviteViaNotification uses an invite notification to accept an invite to a thread
func (t *Textile) AcceptThreadInviteViaNotification(id string) (mh.Multihash, error) {
	// lookup notification
	notification := t.datastore.Notifications().Get(id)
	if notification == nil {
		return nil, errors.New(fmt.Sprintf("could not find notification: %s", id))
	}
	if notification.Type != repo.ReceivedInviteNotification {
		return nil, errors.New(fmt.Sprintf("notification not invite type"))
	}

	// block is the invite's block id
	hash, err := t.AcceptThreadInvite(notification.BlockId, []byte{})
	if err != nil {
		return nil, err
	}

	// delete notification
	return hash, t.datastore.Notifications().Delete(id)
}

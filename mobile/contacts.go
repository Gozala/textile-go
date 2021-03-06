package mobile

import (
	"github.com/pkg/errors"
	"github.com/textileio/textile-go/core"
)

// AddContact calls core AddContact
func (m *Mobile) AddContact(id string, address string, username string) error {
	return m.node.AddContact(id, address, username)
}

// Contact calls core Contact
func (m *Mobile) Contact(id string) (string, error) {
	if !m.node.Started() {
		return "", core.ErrStopped
	}

	contact := m.node.Contact(id)
	if contact != nil {
		return toJSON(contact)
	}
	return "", errors.New("contact not found")
}

// Contacts calls core Contacts
func (m *Mobile) Contacts() (string, error) {
	if !m.node.Started() {
		return "", core.ErrStopped
	}

	contacts, err := m.node.Contacts()
	if err != nil {
		return "", err
	}
	if len(contacts) == 0 {
		contacts = make([]core.ContactInfo, 0)
	}
	return toJSON(contacts)
}

// ContactUsername calls core ContactUsername
func (m *Mobile) ContactUsername(id string) string {
	if !m.node.Started() {
		return ""
	}

	return m.node.ContactUsername(id)
}

// ContactThreads calls core ContactThreads
func (m *Mobile) ContactThreads(id string) (string, error) {
	if !m.node.Started() {
		return "", core.ErrStopped
	}

	infos, err := m.node.ContactThreads(id)
	if err != nil {
		return "", err
	}
	if len(infos) == 0 {
		infos = make([]core.ThreadInfo, 0)
	}
	return toJSON(infos)
}

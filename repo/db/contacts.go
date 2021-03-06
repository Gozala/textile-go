package db

import (
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/textileio/textile-go/repo"
)

type ContactDB struct {
	modelStore
}

func NewContactStore(db *sql.DB, lock *sync.Mutex) repo.ContactStore {
	return &ContactDB{modelStore{db, lock}}
}

func (c *ContactDB) Add(contact *repo.Contact) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert into contacts(id, address, username, inboxes, added) values(?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		log.Errorf("error in tx prepare: %s", err)
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(
		contact.Id,
		contact.Address,
		contact.Username,
		strings.Join(contact.Inboxes, ","),
		int(contact.Added.Unix()),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *ContactDB) AddOrUpdate(contact *repo.Contact) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into contacts(id, address, username, inboxes, added) values(?,?,?,?,coalesce((select added from contacts where id=?),?))`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		log.Errorf("error in tx prepare: %s", err)
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(
		contact.Id,
		contact.Address,
		contact.Username,
		strings.Join(contact.Inboxes, ","),
		contact.Id,
		int(contact.Added.Unix()),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *ContactDB) Get(id string) *repo.Contact {
	c.lock.Lock()
	defer c.lock.Unlock()
	ret := c.handleQuery("select * from contacts where id='" + id + "';")
	if len(ret) == 0 {
		return nil
	}
	return &ret[0]
}

func (c *ContactDB) List() []repo.Contact {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.handleQuery("select * from contacts order by added desc;")
}

func (c *ContactDB) ListByAddress(address string) []repo.Contact {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.handleQuery("select * from contacts where address='" + address + "' order by added desc;")
}

func (c *ContactDB) Count() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	row := c.db.QueryRow("select Count(*) from contacts;")
	var count int
	row.Scan(&count)
	return count
}

func (c *ContactDB) Delete(id string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("delete from contacts where id=?", id)
	return err
}

func (c *ContactDB) handleQuery(stm string) []repo.Contact {
	var ret []repo.Contact
	rows, err := c.db.Query(stm)
	if err != nil {
		log.Errorf("error in db query: %s", err)
		return nil
	}
	for rows.Next() {
		var id, address, username, inboxes string
		var addedInt int
		if err := rows.Scan(&id, &address, &username, &inboxes, &addedInt); err != nil {
			log.Errorf("error in db scan: %s", err)
			continue
		}
		ilist := make([]string, 0)
		for _, p := range strings.Split(inboxes, ",") {
			if p != "" {
				ilist = append(ilist, p)
			}
		}
		ret = append(ret, repo.Contact{
			Id:       id,
			Address:  address,
			Username: username,
			Inboxes:  ilist,
			Added:    time.Unix(int64(addedInt), 0),
		})
	}
	return ret
}

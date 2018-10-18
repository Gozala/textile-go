package db

import (
	"database/sql"
	"github.com/textileio/textile-go/repo"
	"sync"
	"testing"
	"time"
)

var cafeReqDB repo.CafeRequestStore

func init() {
	setupCafeRequestDB()
}

func setupCafeRequestDB() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	cafeReqDB = NewCafeRequestStore(conn, new(sync.Mutex))
}

func TestCafeRequestDB_Add(t *testing.T) {
	err := cafeReqDB.Add(&repo.CafeRequest{
		Id:       "abcde",
		TargetId: "zxy",
		CafeId:   "boom",
		Type:     repo.CafeStoreRequest,
		Date:     time.Now(),
	})
	if err != nil {
		t.Error(err)
	}
	stmt, err := cafeReqDB.PrepareQuery("select id from cafe_requests where id=?")
	defer stmt.Close()
	var id string
	err = stmt.QueryRow("abcde").Scan(&id)
	if err != nil {
		t.Error(err)
	}
	if id != "abcde" {
		t.Errorf(`expected "abcde" got %s`, id)
	}
}

func TestCafeRequestDB_List(t *testing.T) {
	setupCafeRequestDB()
	err := cafeReqDB.Add(&repo.CafeRequest{
		Id:       "abcde",
		TargetId: "zxy",
		CafeId:   "boom",
		Type:     repo.CafeStoreThreadRequest,
		Date:     time.Now(),
	})
	if err != nil {
		t.Error(err)
	}
	err = cafeReqDB.Add(&repo.CafeRequest{
		Id:       "abcdef",
		TargetId: "zxy",
		CafeId:   "boom",
		Type:     repo.CafeStoreRequest,
		Date:     time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Error(err)
	}
	all := cafeReqDB.List("", -1)
	if len(all) != 2 {
		t.Error("returned incorrect number of requests")
		return
	}
	limited := cafeReqDB.List("", 1)
	if len(limited) != 1 {
		t.Error("returned incorrect number of requests")
		return
	}
	offset := cafeReqDB.List(limited[0].Id, -1)
	if len(offset) != 1 {
		t.Error("returned incorrect number of requests")
		return
	}
}

func TestCafeRequestDB_Delete(t *testing.T) {
	err := cafeReqDB.Delete("abcde")
	if err != nil {
		t.Error(err)
	}
	stmt, err := cafeReqDB.PrepareQuery("select id from cafe_requests where id=?")
	defer stmt.Close()
	var id string
	if err := stmt.QueryRow("abcde").Scan(&id); err == nil {
		t.Error("delete failed")
	}
}

func TestCafeRequestDB_DeleteByCafe(t *testing.T) {
	setupCafeRequestDB()
	err := cafeReqDB.Add(&repo.CafeRequest{
		Id:       "xyz",
		TargetId: "zxy",
		CafeId:   "boom",
		Type:     repo.CafeStoreRequest,
		Date:     time.Now(),
	})
	if err != nil {
		t.Error(err)
	}
	err = cafeReqDB.DeleteByCafe("boom")
	if err != nil {
		t.Error(err)
	}
	stmt, err := cafeReqDB.PrepareQuery("select id from cafe_requests where id=?")
	defer stmt.Close()
	var id string
	if err := stmt.QueryRow("zyx").Scan(&id); err == nil {
		t.Error("delete by cafe failed")
	}
}

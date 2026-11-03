package library

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Reading beside writing (ADR-0063). Each library has one connection that
// writes (getDB); a scan, a naming or an edit goes through it, one at a
// time. The statistics, the map, the timeline and the list of libraries
// only read, and they read through a few connections of their own: WAL
// lets readers run beside each other and beside the writer, so the ten
// requests of the Statistics overview no longer wait for each other, and a
// scan no longer holds up a page that only reads.
const (
	readersPerLibrary = 3
	// Each reader keeps its own page cache; three of 16 MB are less than
	// the writer's 64 MB. The database is mapped into memory instead, so the
	// readers share the operating system's pages: a fresh connection read
	// exif_index for 1.4 s before its own cache held it, mapped 0.07 s. The
	// index lies in -lib-dir on a local disk, never on a share, where a
	// mapped file that goes away would stop the program.
	readerCacheKiB = 16384
	readerMapBytes = 2 << 30
	// A reader nobody used for a while is closed, its cache given back.
	readerIdle = 10 * time.Minute
)

// openReadDB opens a pool of connections to a migrated library database
// that cannot write: query_only makes a write an error instead of a second
// writer.
func openReadDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=query_only(1)&_pragma=busy_timeout(5000)&_pragma=cache_size(-%d)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(%d)",
		dbPath, readerCacheKiB, readerMapBytes)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(readersPerLibrary)
	db.SetMaxIdleConns(readersPerLibrary)
	db.SetConnMaxIdleTime(readerIdle)
	return db, nil
}

// getReadDB returns the library's readers, opened after its writer so the
// database is migrated first.
func (m *Manager) getReadDB(id string) (*sql.DB, error) {
	if db, ok := m.readDBs.Load(id); ok {
		return db.(*sql.DB), nil
	}
	if _, err := m.getDB(id); err != nil {
		return nil, err
	}
	m.dbMu.Lock()
	defer m.dbMu.Unlock()
	if db, ok := m.readDBs.Load(id); ok {
		return db.(*sql.DB), nil
	}
	dbPath := filepath.Join(m.LibDir(id), "library.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("library %s not found", id)
	}
	db, err := openReadDB(dbPath)
	if err != nil {
		return nil, err
	}
	m.readDBs.Store(id, db)
	return db, nil
}

// OpenReader returns a Store on the library's readers, for queries that
// only read. A write through it fails.
func (m *Manager) OpenReader(id string) (*Store, error) {
	db, err := m.getReadDB(id)
	if err != nil {
		return nil, err
	}
	store := newStore(db, m.LibDir(id))
	store.decisions = m.decisions
	return store, nil
}

// closeDBs closes the library's writer and readers; the caller holds dbMu.
func (m *Manager) closeDBs(id string) {
	for _, pool := range []*sync.Map{&m.openDBs, &m.readDBs} {
		if db, ok := pool.LoadAndDelete(id); ok {
			forgetLookalikes(db.(*sql.DB))
			db.(*sql.DB).Close()
		}
	}
}

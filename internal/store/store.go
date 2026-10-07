// Package store holds the database handle the engine reads from.
//
// It is internal because the handle belongs to the host application: brokr
// never opens a connection, reads DB_* environment variables or tunes a pool.
// Boot calls Set once; every engine query goes through DB.
package store

import (
	"sync"

	"gorm.io/gorm"
)

var (
	mu sync.RWMutex
	db *gorm.DB
)

// Set installs the database handle the engine will use.
func Set(handle *gorm.DB) {
	mu.Lock()
	defer mu.Unlock()
	db = handle
}

// DB returns the installed handle. It is nil before Boot, which is why every
// exported entry point that reaches the database goes through a booted *Brokr.
func DB() *gorm.DB {
	mu.RLock()
	defer mu.RUnlock()
	return db
}

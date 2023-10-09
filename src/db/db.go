package db

import "github.com/Rosa-Devs/Router/src/types"

type DB struct {
	*types.Config
}

func (db *DB) Start(cfg types.Config) {
	db.Config = &cfg
	spawnWorkers()
}

func spawnWorkers() {

}

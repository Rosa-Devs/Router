package db

import (
	"os"

	"github.com/Rosa-Devs/Router/src/types"
)

type DB struct {
	*types.Config
}

func (db *DB) Start(cfg types.Config) {
	db.Config = &cfg
	spawnWorkers()
}

func spawnWorkers() {

}

func (db *DB) CreateDb(id string) error {
	database_path := db.Config.DatabasePath + "/" + id

	err := os.MkdirAll(database_path, 0775)
	if err != nil {
		return err
	}

	return nil
}

type Database struct {
	db      *DB
	db_name string
}

func (db *DB) GetDb(id string) Database {
	return Database{db: db, db_name: id}
}

func (db *Database) CreatePool(pool_id string) error {
	pool_path := db.db.DatabasePath + "/" + db.db_name + "/" + pool_id

	err := os.Mkdir(pool_path, 0775)
	if err != nil {
		return err
	}

	return nil
}

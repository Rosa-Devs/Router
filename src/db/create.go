package db

import "os"

func (db *DB) CreateDatabase(name string) error {
	//CREATE DATABASE DIRECTORY
	os.Mkdir(db.Config.Database+name, os.ModePerm)

	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Rosa-Devs/Router/src/db"
	"github.com/Rosa-Devs/Router/src/types"
	"github.com/google/uuid"
)

func main() {
	cfg := types.Config{
		DatabasePath: "db",
	}

	// !! GLOBAl DB MANAGER !!
	//CREATE DATABSE INSTANCE
	Drvier := db.DB{}
	//START DATABSE INSTANCE
	Drvier.Start(cfg)
	//CREATE TEST DB
	Drvier.CreateDb("test")

	// !! WORKING WITH SPECIFIED BATABASE !!
	db := Drvier.GetDb("test")

	err := db.CreatePool("test_pool")
	if err != nil {
		log.Println("Mayby this pool alredy exist:", err)
		//return
	}

	pool, err := db.GetPool("test_pool")
	if err != nil {
		log.Println(err)
		return
	}

	// GENERATE RANDOM DATA
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 10; i++ {
		// Generate random data
		randomData := map[string]interface{}{
			"field1": rand.Intn(100),               // Random integer between 0 and 100
			"field2": rand.Float64() * 100,         // Random float between 0 and 100
			"field3": uuid.New().String(),          // Random UUID as a string
			"field4": time.Now().UnixNano(),        // Current timestamp in nanoseconds
			"field5": fmt.Sprintf("Record%d", i+1), // Custom string with record number
		}

		// Convert data to JSON
		jsonData, err := json.Marshal(randomData)
		if err != nil {
			fmt.Println("Error marshaling JSON:", err)
			return
		}

		// Call Record function to save the record
		err = pool.Record(jsonData)
		if err != nil {
			fmt.Println("Error recording data:", err)
			return
		}
	}

	pool.Tree()

	data, err := pool.GetByID("e89cd617-269b-4553-bbdc-98a430f6cbfe")
	if err != nil {
		log.Println("Err:", err)
	}
	fmt.Println(data["field2"])

}

package database

import (
	"database/sql"
	"fmt"
	"log"
	"otter/models"
	"strconv"

	_ "github.com/marcboeker/go-duckdb"
)

type Database struct {
	c *sql.DB // the "connection" object - (the database, loaded in memory)
}

func Connect() Database {
	db, err := sql.Open("duckdb", "signals.duckdb")
	if err != nil {
		log.Fatal(err)
	}

	return Database{
		c: db,
	}
}

func (db *Database) GetSimulationStartAndEnd() (int64, int64, error) {
	var (
		simulation_start int64
		simulation_end   int64
	)

	row := db.c.QueryRow(`SELECT min(timestamp) as simulation_start, max(timestamp) as simulation_end FROM events`)

	err := row.Scan(&simulation_start, &simulation_end)

	if err != nil {
		return 0, 0, err
	}

	return simulation_start, simulation_end, nil
}

// @info The CA Map is a map that allows resolving file_id -> CA. file_id is the primary key in the metadata table.
func (db *Database) GetContractAddressInfo() (map[int]models.Asset, error) {
	rows, err := db.c.Query(`SELECT file_id, ca, CAST(call_timestamp AS BIGINT), name, description, image_uri FROM file_metadata;`)
	if err != nil {
		fmt.Println(err)
	}

	var assets = make(map[int]models.Asset, 0)

	var descriptionP, nameP, imageURLP sql.NullString

	for rows.Next() {
		var a models.Asset
		if err := rows.Scan(&a.FileID, &a.ContractAddress, &a.CallTimestamp, &nameP, &descriptionP, &imageURLP); err != nil {
			fmt.Println(err)
			return assets, nil
		}

		a.Price = 0.0
		a.Balance = 0.0

		a.TPStage = 0
		a.EntryPrice = 0.0

		if descriptionP.Valid {
			a.Description = descriptionP.String
		} else {
			a.Description = ""
		}

		if nameP.Valid {
			a.Name = nameP.String
		} else {
			a.Name = ""
		}

		if imageURLP.Valid {
			a.ImageURL = imageURLP.String
		} else {
			a.ImageURL = ""
		}

		a.TradingHistory = make(map[int64]float64, 0)
		a.TPPrice = 0.0

		assets[a.FileID] = a
	}

	if err := rows.Err(); err != nil {
		return assets, err
	}

	return assets, nil
}

// @deprecated - only iused when batchrequesting events for timestamps is disabled
func (db *Database) EventsOccuringAtTimestamp(timestamp int64) ([]models.Event, error) {
	rows, err := db.c.Query(`SELECT * FROM events WHERE timestamp = ` + strconv.FormatInt(timestamp, 10))
	if err != nil {
		fmt.Println(err)
	}

	var events []models.Event

	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.FileID, &e.EventDisplayType, &e.QuoteToken, &e.SOLPrice, &e.TokenPrice, &e.Timestamp, &e.BlockNumber); err != nil {
			return events, nil
		}

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return events, err
	}

	return events, nil
}

func (db *Database) BatchGetEventsForTimestamps(timestamp int64, batch_size int64) ([]models.Event, error) {
	rows, err := db.c.Query(`SELECT * FROM events WHERE timestamp >= ` + strconv.FormatInt(timestamp, 10) + ` AND timestamp < ` + strconv.FormatInt(timestamp+batch_size, 10) + `;`)
	if err != nil {
		fmt.Println(err)
	}

	var events []models.Event

	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.FileID, &e.EventDisplayType, &e.QuoteToken, &e.SOLPrice, &e.TokenPrice, &e.Timestamp, &e.BlockNumber); err != nil {
			return events, nil
		}

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return events, err
	}

	return events, nil
}

func (db *Database) Disconnect() {
	db.c.Close()
}

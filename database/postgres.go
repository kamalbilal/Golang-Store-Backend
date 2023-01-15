package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "Kamal1675."
	dbname   = "shop"
)

// ConnectToDatabase creates a connection to the PostgreSQL database
func ConnectToDatabase() (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}
	
	// Set the maximum number of connections in the pool
	db.SetMaxOpenConns(10)

	// Set the maximum number of idle connections in the pool
	db.SetMaxIdleConns(5)

	return db, nil
}


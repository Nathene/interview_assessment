package database

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

type DbRequest struct {
	query  string
	args   []interface{}
	result chan DbResult
}

type DbResult struct {
	rows *sql.Rows
	err  error
}

type database struct {
	db      *sql.DB
	queries chan DbRequest
}

func init() {
	sqlDB, err := sql.Open("sqlite", "interview.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create sessions table with all columns
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			candidate_id TEXT,
			candidate_name TEXT,
			candidate_email TEXT,
			candidate_rating INTEGER,
			metadata_interview_time TEXT,
			metadata_duration INTEGER,
			metadata_interview_type TEXT,
			metadata_status TEXT,
			metadata_link TEXT,
			metadata_date TEXT,
			metadata_timezone TEXT,
			created_by TEXT REFERENCES users(id),
			feedback TEXT,            
			notes TEXT                 
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Create users table if it doesn't exist
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			password_hash TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	db = &database{
		db:      sqlDB,
		queries: make(chan DbRequest),
	}
	go db.listen()
}

var db *database

// QueryRow sends a query and returns a single row result
func (db *database) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.db.QueryRow(query, args...)
}

// Query sends a query through the actor model and returns the result
func Query(query string, args ...interface{}) (*sql.Rows, error) {
	resultChan := make(chan DbResult)
	req := DbRequest{
		query:  query,
		args:   args,
		result: resultChan,
	}

	db.queries <- req
	result := <-resultChan
	return result.rows, result.err
}

// Exec sends an execution query through the actor model
func (db *database) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.db.Exec(query, args...)
}

func (db *database) listen() {
	for req := range db.queries {
		rows, err := db.db.Query(req.query, req.args...)
		req.result <- DbResult{rows: rows, err: err}
	}
}

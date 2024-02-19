package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// Функция для работы с бд по заданию(insert и select)
func DBaction(reqAm int, decodedRequestList RequestList, resArr ResponseList) {
	var DBrequests []DBrequest
	var curDBrequest DBrequest
	for i := range reqAm {
		curDBrequest.Url = decodedRequestList.Requests[i].Url
		curDBrequest.Method = decodedRequestList.Requests[i].Method
		curDBrequest.ResponseCode = resArr.Responses[i].ResponseCode
		DBrequests = append(DBrequests, curDBrequest)
	}

	connStr := "postgres://postgres:admin@localhost:5432/postgres?sslmode=disable"

	db, err := sql.Open("postgres", connStr)

	defer db.Close()

	if err != nil {
		fmt.Println(err)
		return
	}

	if err = db.Ping(); err != nil {
		fmt.Println(err)
		return
	}

	valid := createTable(db)
	if valid == false {
		return
	}

	start := -1
	var end int

	for _, DBreq := range DBrequests {
		c := insertRequest(db, DBreq)
		if c == -1 {
			return
		}
		if start == -1 {
			start = c
		}
		end = c
	}

	query := `SELECT url, method, responsecode FROM requests WHERE id = $1`
	var url string
	var method string
	var responsecode int
	for c := start; c <= end; c++ {
		err = db.QueryRow(query, c).Scan(&url, &method, &responsecode)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(c, url, method, responsecode)
	}
}

func createTable(db *sql.DB) bool {
	query := `
	CREATE TABLE IF NOT EXISTS requests (
		id SERIAL PRIMARY KEY,
		url VARCHAR(256),
		method VARCHAR(256),
		responseCode INT
	);
	`

	_, err := db.Exec(query)
	if err != nil {
		fmt.Println(err)
		return false
	}

	return true
}

func insertRequest(db *sql.DB, request DBrequest) int {
	query := `
	INSERT INTO requests (url, method, responsecode)
		VALUES ($1, $2, $3) RETURNING id
		`

	var pk int

	err := db.QueryRow(
		query,
		request.Url,
		request.Method,
		request.ResponseCode).
		Scan(&pk)
	if err != nil {
		fmt.Println(err)
		return -1
	}

	return pk
}

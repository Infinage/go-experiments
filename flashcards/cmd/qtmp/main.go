package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "tmp/sanity.db")
	rows, err := db.Query(`select id, front_text, length(front_image), front_image_mime, length(front_audio) from cards where front_text='media roundtrip'`)
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		var i, li, la int
		var f string
		var m sql.NullString
		rows.Scan(&i, &f, &li, &m, &la)
		fmt.Println(i, f, li, m.String, la)
	}
}

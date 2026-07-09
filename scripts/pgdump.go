package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "host=localhost port=5432 user=newapi password=adaf5c94692108bc dbname=new-api sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ping failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("connected")

	for _, table := range []string{"users", "tokens", "channels", "abilities"} {
		rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s ORDER BY id LIMIT 3", table))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", table, err)
			continue
		}
		cols, _ := rows.Columns()
		ncols := len(cols)
		vals := make([]interface{}, ncols)
		vptrs := make([]interface{}, ncols)
		for i := range vals {
			vptrs[i] = &vals[i]
		}

		count := 0
		for rows.Next() {
			rows.Scan(vptrs...)
			count++
		}
		rows.Close()

		fmt.Printf("%s: %d cols, %d rows\n", table, ncols, count)
	}
}

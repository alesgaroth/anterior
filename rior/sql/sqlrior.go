package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"alesgaroth.com/anterior/rior"
)

type RiorSqlConnection struct {
	db *sql.DB
}

func (rsc *RiorSqlConnection) Close() {
	rsc.db.Close()
}

func (rsc *RiorSqlConnection) Ping() error {
	return rsc.db.Ping()
}

func (rsc *RiorSqlConnection) Query1(query string) (rior.Rior, error) {
	rows, err := rsc.db.Query(query)
	if err == nil {
		defer rows.Close()
		rows.Next()
		columns, err := rows.Columns()
		if err == nil {
			num := len(columns)
			arr := make([]any, 0)
			arr2 := make([]*string, 0)
			for k := 0; k < num; k += 1 {
				var name string
				arr = append(arr, &name)
				arr2 = append(arr2, &name)
			}
			err = rows.Scan(arr...)
			if err == nil {
				data  := make(map[string]string)
				for k, colname := range columns {
					data[colname] = *arr2[k]
				}
				return &SQL1Rior{data}, nil
			} else {
				return nil, fmt.Errorf("yo1", err)
			}
		} else {
			return nil, fmt.Errorf("yo2", err)
		}
	} else {
		return nil, fmt.Errorf("yo3 %v", err)
	}
	return nil, err
}
func (rsc *RiorSqlConnection) Query(query string) (rior.Rior, error) {

	var err error
	if rows, err := rsc.db.Query(query); err != nil {
		if columns, err := rows.Columns(); err != nil {
			return &SQLRior{rows, columns}, nil
		}
	}
	return nil, err
}

type SQL1Rior struct {
	data map[string]string
}
func (sr *SQL1Rior) Get(name string) string {
	if dit, ok := sr.data[name]; ok {
		return dit
	}
	return ""
}
func (sr *SQL1Rior) GetDS(name string) rior.Rior {
	return nil
}

type SQLRior struct {
	rows *sql.Rows
	columns []string
}
func (sr *SQLRior) Get(name string) string {
	return ""
}
func (sr *SQLRior) GetDS(name string) rior.Rior {
	return nil
}


func NewRiorSqlConnection(username, hostname, database string) (*RiorSqlConnection, error) {
	connectionString := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", username, username, hostname, database)
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}
	return &RiorSqlConnection{db}, nil
}

func main() {
	sqlConn, err := NewRiorSqlConnection("alesgaroth", "localhost", "weblog", )


	if err != nil {
		log.Fatal(err)
	}
	defer sqlConn.Close()

	err = sqlConn.Ping()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Successfully connected to PostgreSQL!")

  var ds rior.Rior
	ds, err = sqlConn.Query1("SELECT * FROM post")
	if err != nil {
		log.Fatal(err)
	}
	if ds == nil {
		log.Fatal("oops")
	}
	fmt.Printf("id : %v title %v body %v\n", ds.Get("id"), ds.Get("title"), ds.Get("body"))

	db := sqlConn.db
	rows, err := db.Query("SELECT * FROM post")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close() // Process query results
	for rows.Next() {
		var id int
		var title string
		var body string
		// Process each row

		err := rows.Scan(&id, &title, &body)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("id : %d title %s body %s\n", id, title, body)

	}
}

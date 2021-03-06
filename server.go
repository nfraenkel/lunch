package main

import (
	_ "database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/nfraenkel/lunch/Godeps/_workspace/src/github.com/jmoiron/sqlx"
	_ "github.com/nfraenkel/lunch/Godeps/_workspace/src/github.com/lib/pq"
	"github.com/nfraenkel/lunch/Godeps/_workspace/src/github.com/zenazn/goji"
	"github.com/nfraenkel/lunch/Godeps/_workspace/src/github.com/zenazn/goji/web"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Server struct {
	db   *sqlx.DB
	port string
}

var server Server

type ApiHandlerFunc func(web.C, http.ResponseWriter, *http.Request) (int, error)

func ApiHandler(h ApiHandlerFunc) web.HandlerFunc {
	return web.HandlerFunc(func(c web.C, w http.ResponseWriter, r *http.Request) {
		code, err := h(c, w, r)
		if err != nil {
			http.Error(w, err.Error(), code)
		} else {
			w.WriteHeader(code)
		}
	})
}

func initServer() {
	var migrationsFolder string
	var dbUrl string
	var migrationSql = `
	CREATE TABLE IF NOT EXISTS migrations (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		created TIMESTAMP without time zone default (now() at time zone 'utc') 
	)`

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
		dbUrl = devDb
	} else {
		dbUrl = os.Getenv("DATABASE_URL")
	}
	flag.Set("bind", ":"+port)
	serverDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	migrationsFolder = filepath.Join(serverDir, "./migrations")

	db, err := sqlx.Connect("postgres", dbUrl)
	if err != nil {
		log.Fatal(err)
	}
	Log("executing migrations\n")
	server.db = db
	server.db.MustExec(migrationSql)

	d, err := os.Open(migrationsFolder)
	if err != nil {
		log.Fatal(err)
	}
	dir, err := d.Readdir(-1)
	if err != nil {
		log.Fatal(err)
	}

	sqlFiles := make([]string, 0)
	for _, f := range dir {
		ext := filepath.Ext(f.Name())
		if ".sql" == ext {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)
	for _, filename := range sqlFiles {
		migrated, err := HasMigrated(filename)
		if err != nil {
			server.db.Close()
			log.Fatal(err)
		}
		fullpath := filepath.Join(migrationsFolder, filename)
		if migrated {
			continue
		}
		b, err := ioutil.ReadFile(fullpath)
		if err != nil {
			server.db.Close()
			log.Fatal(err)
		}
		migration := string(b)
		if len(migration) == 0 {
			Log(fmt.Sprintf("skipping empty file %s", filename))
			continue
		}
		server.db.MustExec(migration)
		server.db.MustExec("INSERT INTO migrations (name) values ($1)", filename)
		Log(fmt.Sprintf("migrated file %s", filename))
	}

	if err != nil {
		server.db.Close()
		log.Fatal(err)
	}

}

func HasMigrated(filename string) (bool, error) {
	var count int
	err := server.db.QueryRow("select count(1) from migrations where name = $1", filename).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

type User struct {
	Id      int       `db:"user_id" param:"user_id" json:"id"`
	First   string    `db:"user_first" param:"user_first" json:"first_name"`
	Last    string    `db:"user_last" param:"user_last" json:"last_name"`
	Email   string    `db:"user_email" param:"user_email" json:"email"`
	Photo   string    `db:"user_photo" param:"user_photo" json:"photo"`
	Created time.Time `db:"user_created" param:"user_created" json:"created"`
}

func GetUser(email string) (*User, error) {
	u := &User{Email: email}
	err := server.db.QueryRow(`
		SELECT 
		user_id, user_first_name, user_last_name, user_photo
		FROM users WHERE user_email = $1`, u.Email).Scan(&u.Id, &u.First, &u.Last, &u.Photo)
	return u, err
}

func (u *User) Create() error {
	return server.db.QueryRow(`
		INSERT INTO users 
			(user_first_name, user_last_name, user_photo, user_email, user_created)
			VALUES ($1, $2, $3, $4, $5) RETURNING user_id
		`, u.First, u.Last, u.Photo, u.Email, time.Now().UTC()).Scan(&u.Id)
}

func (u *User) CreateIfDoesntExist() error {
	var count int
	err := server.db.QueryRow(`SELECT count(*) FROM users WHERE user_email = $1`, u.Email).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return u.Create()
}

type Venue struct {
	Id       int       `db:"venue_id" param:"venue_id" json:"id"`
	Name     string    `db:"venue_name" param:"venue_name" json:"name"`
	Photo    string    `db:"venue_photo" param:"venue_photo" json:"photo"`
	Location string    `db:"venue_location" param:"venue_location" json:"location"`
	Distance string    `db:"venue_distance" param:"venue_distance" json:"distance"`
	Type     string    `db:"venue_type" param:"venue_type" json:"type"`
	Created  time.Time `db:"venue_created" param:"venue_created" json:"created"`
}

func (v *Venue) Create() error {
	return server.db.QueryRow(`
		INSERT INTO venues 
			(venue_name, venue_photo, venue_location, venue_type, venue_distance, venue_created)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING venue_id
		`, v.Name, v.Photo, v.Location, v.Type, v.Distance, time.Now().UTC()).Scan(&v.Id)
}

func (v *Venue) CreateIfDoesntExist() error {
	var count int
	err := server.db.QueryRow(`SELECT count(*) FROM venues WHERE venue_name = $1`, v.Name).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return v.Create()
}

type Choice struct {
	User    int       `db:"user_id" param:"user_id" json:"user_id"`
	Venue   int       `db:"venue_id" param:"venue_id" json:"venue_id"`
	Created time.Time `db:"choice_created" param:"choice_created" json:"created"`
}

func (c *Choice) Create() error {
	_, err := server.db.Exec(`DELETE FROM choices WHERE user_id = $1`, c.User)
	if err != nil {
		return err
	}
	return server.db.QueryRow(`
		INSERT INTO choices 
			(user_id, venue_id)
			VALUES ($1, $2) RETURNING choice_created
		`, c.User, c.Venue).Scan(&c.Created)
}

func (c *Choice) Delete() error {
	_, err := server.db.Exec(`DELETE FROM choices WHERE user_id = $1`, c.User)
	return err
}

type LoginPayload struct {
	Email string `json:"email"`
}

func Login(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	var err error
	l := &LoginPayload{}
	decoder := json.NewDecoder(r.Body)
	if err = decoder.Decode(l); err != nil {
		return http.StatusNotAcceptable, err
	}
	u, err := GetUser(l.Email)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return respond(w, u)
}

func GetUsers(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	var buf []byte
	getUsersSql := `
		SELECT json_agg(x) from (
			SELECT
				u.user_id as id,
				u.user_first_name as first_name,
				u.user_email as email,
				u.user_last_name as last_name,
				u.user_photo as photo
			FROM users u
		) x;
	`
	rows, err := server.db.Query(getUsersSql)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	for rows.Next() {
		rows.Scan(&buf)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf)
	return http.StatusOK, nil
}

func DeleteChoice(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	choice := &Choice{}
	var err error
	decoder := json.NewDecoder(r.Body)
	if err = decoder.Decode(choice); err != nil {
		return http.StatusNotAcceptable, err
	}
	err = choice.Delete()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func GetVenuesWithChoices(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	var buf []byte
	getVenuesSql := `
		SELECT json_agg(x) from (
			SELECT
				v.venue_id as id,
				v.venue_name as name,
				v.venue_location as location,
				v.venue_distance as distance,
				v.venue_type as type,
				v.venue_photo as photo,
				(SELECT coalesce(
					json_agg(
						json_build_object(
							'id', u.user_id,
							'first_name', u.user_first_name,
							'email', u.user_email,
							'last_name', u.user_last_name,
							'photo', u.user_photo
							)
						),
						json'[]')
					FROM users u
					JOIN choices c USING (user_id)
					WHERE c.venue_id = v.venue_id
				) users,
				(SELECT count(*) FROM users u JOIN choices c USING (user_id) WHERE c.venue_id = v.venue_id) AS count_num
			FROM venues v ORDER BY count_num DESC
		) x;
	`
	rows, err := server.db.Query(getVenuesSql)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	for rows.Next() {
		rows.Scan(&buf)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf)
	return http.StatusOK, nil
}

func hello(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	fmt.Fprintf(w, "Hello, %s!", c.URLParams["name"])
	return http.StatusOK, populateHistory()
}

func PopulateDb(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	populateDb()
	populateDbWithUsers()
	return http.StatusOK, nil
}

func PopulateHistory(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	return http.StatusOK, populateHistory()
}

func respond(w http.ResponseWriter, v interface{}) (int, error) {
	w.Header().Set("Content-Type", "application/json")
	buf, err := json.Marshal(v)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Write(buf)
	return http.StatusOK, nil
}

func CreateVenue(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	venue := &Venue{}
	var err error
	decoder := json.NewDecoder(r.Body)
	if err = decoder.Decode(venue); err != nil {
		return http.StatusNotAcceptable, err
	}
	if err = venue.Create(); err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	buf, _ := json.Marshal(venue)
	w.Write(buf)
	return http.StatusOK, nil
}

func CreateChoice(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	choice := &Choice{}
	var err error
	decoder := json.NewDecoder(r.Body)
	if err = decoder.Decode(choice); err != nil {
		return http.StatusNotAcceptable, err
	}
	if err = choice.Create(); err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	buf, _ := json.Marshal(choice)
	w.Write(buf)
	return http.StatusOK, nil
}

func populateHistory() error {
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	venueMap := make(map[int][]int)
	choices := []Choice{}
	err := server.db.Select(&choices, `SELECT * FROM choices WHERE choice_created > $1`, yesterday)
	if err != nil {
		return err
	}
	for _, c := range choices {
		venueMap[c.Venue] = append(venueMap[c.Venue], c.User)
	}
	var venueName string
	var userNames string
	for venueId, users := range venueMap {
		err = server.db.QueryRow(`SELECT venue_name FROM venues WHERE venue_id=$1`, venueId).Scan(&venueName)
		if err != nil {
			return err
		}
		q,args,err := sqlx.In(`SELECT string_agg(user_first_name, ', ') FROM users WHERE user_id IN (?)`, users)
		if err != nil {
			return err
		}		
		q = sqlx.Rebind(sqlx.DOLLAR,q)
		err = server.db.QueryRow(q,args...).Scan(&userNames)
		if err != nil {
			return err
		}		
		text := userNames + " went to " + venueName
		fmt.Println(text)
		_, err = server.db.Exec(`INSERT INTO history (venue_id, history_text) VALUES ($1, $2)`, venueId, text)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetChoiceHistory(c web.C, w http.ResponseWriter, r *http.Request) (int, error) {
	var buf []byte
	getChoiceHistory := `
		SELECT json_agg(x) from (
			SELECT
				h.history_id as id,
				h.history_text as text,
				(SELECT json_build_object(
					'id', v.venue_id,
					'name', v.venue_name,
					'photo', v.venue_photo
					) from venues v WHERE v.venue_id = h.venue_id
				) AS venue,
				h.history_created as created
			FROM history h
		) x;
	`
	rows, err := server.db.Query(getChoiceHistory)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	for rows.Next() {
		rows.Scan(&buf)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf)
	return http.StatusOK, nil
}

func main() {
	initServer()
	goji.Get("/hello/:name", ApiHandler(hello))
	goji.Post("/api/login", ApiHandler(Login))
	goji.Get("/api/venues", ApiHandler(GetVenuesWithChoices))
	goji.Get("/api/users", ApiHandler(GetUsers))
	goji.Delete("/api/choices", ApiHandler(DeleteChoice))
	goji.Get("/api/history", ApiHandler(GetChoiceHistory))
	goji.Post("/api/venues", ApiHandler(CreateVenue))
	goji.Post("/api/choices", ApiHandler(CreateChoice))
	goji.Post("/api/populate-db", ApiHandler(PopulateDb))
	goji.Post("/api/populate-history", ApiHandler(PopulateHistory))
	goji.Serve()
}

func Log(message string) {
	fmt.Println(message)
}

func populateDb() {
	csvfile, err := os.Open("venues.csv")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer csvfile.Close()
	reader := csv.NewReader(csvfile)
	reader.FieldsPerRecord = -1 // see the Reader struct information below
	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, each := range rawCSVdata {
		v := &Venue{
			Name:     each[1],
			Photo:    each[5],
			Type:     each[2],
			Distance: each[3],
			Location: "New York, NY",
		}
		err = v.CreateIfDoesntExist()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("name : %s and pic : %s\n", each[1], each[5])
	}
}

func populateDbWithUsers() {
	csvfile, err := os.Open("users.csv")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer csvfile.Close()
	reader := csv.NewReader(csvfile)
	reader.FieldsPerRecord = -1 // see the Reader struct information below
	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, each := range rawCSVdata {
		u := &User{
			First: each[1],
			Last:  each[2],
			Email: each[3],
			Photo: each[4],
		}
		err = u.CreateIfDoesntExist()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("name : %s and pic : %s\n", each[1], each[4])
	}
}

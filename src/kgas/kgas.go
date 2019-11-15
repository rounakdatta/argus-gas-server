package main

import (
	// "strings"
	"encoding/json"
	"database/sql"
	"fmt"
	"os"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// LevelStruct is a struct for holding the [current, maximum] level
type LevelStruct struct {
	Current float32 `json:"current"`
	Maximum float32 `json:"maximum"`
}

// StatusStruct is a struct for holding the [current, maximum] level, deviceId, wirelessNetwork, lastSeen
type StatusStruct struct {
	CurrentLevel float32 `json:"currentLevel"`
	MaximumLevel float32 `json:"maximumLevel"`
	DeviceID string `json:"deviceId"`
	WirelessNetwork string `json:"wirelessNetwork"`
	LastSeen string `json:"lastSeen"`
}

var db *sql.DB

func main() {
	// connect to MySQL database
	err := godotenv.Load()

	databaseCredentials := fmt.Sprintf("%s:%s@/%s", os.Getenv("APP_USER"), os.Getenv("APP_PASSWORD"), os.Getenv("APP_DATABASE"))
	db, err = sql.Open("mysql", databaseCredentials)

	if err != nil {
		panic(err.Error())
	}

	defer db.Close()

	// create the router and define the APIs
	router := mux.NewRouter()
	kgasRouter := router.PathPrefix("/kgas").Subrouter()

	kgasRouter.HandleFunc("/", GetRoot).Methods("GET")
	kgasRouter.HandleFunc("/api/update/level/", UpdateLevel).Methods("POST")
	kgasRouter.HandleFunc("/api/get/status/", GetStatus).Methods("GET")
	kgasRouter.HandleFunc("/api/get/level/", GetLevel).Methods("GET")

	http.Handle("/", router)


	log.Println("Server started on port 999")
	log.Fatal(http.ListenAndServe(":999", handlers.CORS(handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}), handlers.AllowedMethods([]string{"GET", "POST", "PUT", "HEAD", "OPTIONS"}), handlers.AllowedOrigins([]string{"*"}))(router)))
}

// GetRoot returns OK if the server is alive
func GetRoot(w http.ResponseWriter, r *http.Request) {
	payload := []byte("OK")
	w.Write(payload)
}

// GetLevel returns the current level
func GetLevel(w http.ResponseWriter, r *http.Request) {
	customerID, ok := r.URL.Query()["c"]

	if ok && customerID != nil {
		getLevelQuery := fmt.Sprintf(`SELECT currentWeight, maxWeight FROM levelMeter
			WHERE customerId='%s'
		`, customerID[0])

		var currentWeight float32
		var maxWeight float32
		err := db.QueryRow(getLevelQuery).Scan(&currentWeight, &maxWeight)
		if err != nil {
			payload := []byte("Invalid Argument")
			w.Write(payload)

			return
		}

		responseData := LevelStruct{
			Current: currentWeight,
			Maximum: maxWeight,
		}

		payloadJSON, err := json.Marshal(responseData)
		if err != nil {
			log.Println(err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(payloadJSON)

		return
	}

	payload := []byte("Request Arguments(s) missing")
	w.Write(payload)
}

// GetStatus returns the current level
func GetStatus(w http.ResponseWriter, r *http.Request) {
	customerID, ok := r.URL.Query()["c"]

	if ok && customerID != nil {
		getLevelQuery := fmt.Sprintf(`SELECT currentWeight, maxWeight, deviceId, wirelessNetwork, lastSeen FROM levelMeter
			WHERE customerId='%s'
		`, customerID[0])

		var currentWeight float32
		var maxWeight float32
		var deviceID string
		var wirelessNetwork string
		var lastSeen string

		err := db.QueryRow(getLevelQuery).Scan(&currentWeight, &maxWeight, &deviceID, &wirelessNetwork, &lastSeen)
		if err != nil {
			payload := []byte("Invalid Argument")
			w.Write(payload)

			return
		}

		responseData := StatusStruct{
			CurrentLevel: currentWeight,
			MaximumLevel: maxWeight,
			DeviceID: deviceID,
			WirelessNetwork: wirelessNetwork,
			LastSeen: lastSeen,
		}

		payloadJSON, err := json.Marshal(responseData)
		if err != nil {
			log.Println(err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(payloadJSON)

		return
	}

	payload := []byte("Request Arguments(s) missing")
	w.Write(payload)
}

// UpdateLevel returns success if the table updation with the latest value is successful
func UpdateLevel(w http.ResponseWriter, r *http.Request) {

	customerID := r.FormValue("customerId")
	levelValue := r.FormValue("level")

	levelUpdateQuery := fmt.Sprintf(`UPDATE levelMeter
		SET
			currentWeight=%s
		WHERE
			customerId='%s'
	`, levelValue, customerID)


	_, err := db.Query(levelUpdateQuery)

	var result map[string]bool

	if err != nil {
		result = map[string]bool {
			"success": false,
		}
	} else {
		result = map[string]bool {
			"success": true,
		}
	}

	payloadJSON, err := json.Marshal(result)
	if err != nil {
		log.Println(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(payloadJSON)
}

package main

import (
	"strings"
	"encoding/json"
	"database/sql"
	"fmt"
	"os"
	"log"
	"net/http"
	"strconv"

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

	// routes for customer-related
	kgasRouter.HandleFunc("/api/register/customer/", RegisterCustomer).Methods("POST")

	// routes for level-related
	kgasRouter.HandleFunc("/api/update/level/", UpdateLevel).Methods("POST")
	kgasRouter.HandleFunc("/api/get/status/", GetStatus).Methods("GET")
	kgasRouter.HandleFunc("/api/get/level/", GetLevel).Methods("GET")

	// routes for alerts-related
	kgasRouter.HandleFunc("/api/set/alert/", SetNewAlert).Methods("POST")

	http.Handle("/", router)


	log.Println("Server started on port 999")
	log.Fatal(http.ListenAndServe(":999", handlers.CORS(handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}), handlers.AllowedMethods([]string{"GET", "POST", "PUT", "HEAD", "OPTIONS"}), handlers.AllowedOrigins([]string{"*"}))(router)))
}

// GetRoot returns OK if the server is alive
func GetRoot(w http.ResponseWriter, r *http.Request) {
	payload := []byte("OK")
	w.Write(payload)
}

// RegisterCustomer registers the customer to be listening to a device
func RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	deviceID := r.FormValue("deviceId")
	customerID := r.FormValue("customerId")
	maxWeight := r.FormValue("maxWeight")

	registerCustomerQuery := fmt.Sprintf(`INSERT INTO levelMeter (customerId, maxWeight, deviceId)
		VALUES ('%s', %s, '%s')`, customerID, maxWeight, deviceID)

	_, err := db.Query(registerCustomerQuery)
	var result map[string]bool

	if err != nil {
		log.Println(err);
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

// GetLevel returns the current level
func GetLevel(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := r.URL.Query()["d"]

	if ok && deviceID != nil {
		getLevelQuery := fmt.Sprintf(`SELECT currentWeight, maxWeight FROM levelMeter
			WHERE deviceId='%s'
		`, deviceID[0])

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

	deviceID := r.FormValue("deviceId")
	levelValue := r.FormValue("level")

	levelUpdateQuery := fmt.Sprintf(`UPDATE levelMeter
		SET
			currentWeight=%s
		WHERE
			deviceId='%s'
	`, levelValue, deviceID)


	_, err := db.Query(levelUpdateQuery)

	var result map[string]bool

	if err != nil {
		log.Println(err);
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

	TriggerAlert("ROUNAK123", levelValue)
}

// SetNewAlert creates a new alert registered
func SetNewAlert(w http.ResponseWriter, r *http.Request) {

	customerID := r.FormValue("customerId")
	deviceID := r.FormValue("deviceId")
	alertLevel := r.FormValue("alertLevel")

	// get the max weight
	getMaximumLevelQuery := fmt.Sprintf(`SELECT maxWeight FROM levelMeter WHERE customerId='%s'`, customerID)
	var maximumLevelValueString string
	db.QueryRow(getMaximumLevelQuery).Scan(&maximumLevelValueString)

	alertLevelPercentage, _ := strconv.ParseFloat(alertLevel, 32)
	maximumLevelValue, _ := strconv.ParseFloat(maximumLevelValueString, 32)

	alertLevelValue := (alertLevelPercentage * maximumLevelValue) / 100;

	newAlertQuery := fmt.Sprintf(`INSERT INTO levelAlerts
		(customerId, deviceId, alertLevelPercentage, alertLevelValue)
		VALUES ('%s', '%s', %s, %f)
	`, customerID, deviceID, alertLevel, alertLevelValue)

	_, err := db.Query(newAlertQuery)

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

// TriggerAlert triggers an alert if set
func TriggerAlert(customerID string, currentLevel string) {
	nearestAlertQuery := fmt.Sprintf(`SELECT deviceId, alertLevelPercentage FROM levelAlerts
		WHERE alertLevelValue <= (%s + 10) AND alertLevelValue >= %s
		ORDER BY alertLevelValue DESC LIMIT 1`, currentLevel, currentLevel)

	var deviceID string
	var desiredLevel string
	err := db.QueryRow(nearestAlertQuery).Scan(&deviceID, &desiredLevel)

	if err != nil {
		return
	}

	desiredLevelFloat, _ := strconv.ParseFloat(desiredLevel, 32)

	log.Println("Now sending notification")
	CreateLevelAlertNoti(deviceID, desiredLevelFloat)
}

// CreateLevelAlertNoti creates a custom message for a particular user
func CreateLevelAlertNoti(deviceID string, level float64) {
	emoji := "🔔"

	// logic to decide emoji
	if (level >= 75) {
		emoji = "💯"
	} else if (level >= 40 && level < 75) {
		emoji = "🙂"
	} else if (level >= 20 && level < 40) {
		emoji = "😟"
	} else {
		emoji = "😣"
	}

	notiHeading := "Kezpo Gas Level Alert"
	notiContent := fmt.Sprintf(`Your propane level is now %.2f%s %s`, level, "%", emoji)

	SendNoti(notiHeading, notiContent)
	log.Println("Notification sending success: " + deviceID)
} 

// SendNoti sends a push notification to a particular client
func SendNoti(notiHeading string, notiContent string) {

	baseURL := "https://onesignal.com/api/v1/notifications"
	payload := strings.NewReader(fmt.Sprintf("{\"app_id\": \"185a1a32-b95b-4b9f-b752-b9ed84ee3d73\", \"headings\": { \"en\": \"%s\"}, \"contents\": {\"en\": \"%s\"}, \"included_segments\": [\"Subscribed Users\"]}", notiHeading, notiContent))

	req, _ := http.NewRequest("POST", baseURL, payload)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	req.Header.Add("Authorization", "Basic YzAzMTVkMzgtYjQzMy00YjhhLTk0Y2ItY2Y3MzIzZTdkNWRi")

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Println(err)
		return
	}

	defer res.Body.Close()
	// body, _ := ioutil.ReadAll(res.Body)

	// fmt.Println(res)
	// fmt.Println(string(body))
} 
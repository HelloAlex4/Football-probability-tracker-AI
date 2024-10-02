package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./FootballTracker.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	return nil
}

func getDB() *sql.DB {
	return db
}

func closeDB() {
	if db != nil {
		db.Close()
	}
}

func checkIfRowExists(table string, column string, value float64) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE %s = ?", table, column)
	var exists int
	err := getDB().QueryRow(query, value).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func enterDataIntoDB(table string, columns []string, data []interface{}) error {
	// Create placeholders for the SQL query
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	// Construct the SQL query
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	// Execute the query
	_, err := getDB().Exec(query, data...)
	if err != nil {
		return fmt.Errorf("failed to insert data: %v", err)
	}
	return nil
}

func getFixturesForYear(year string) []interface{} {
	url := fmt.Sprintf("https://v3.football.api-sports.io/fixtures?season=%s&league=207", year)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	apiKey := os.Getenv("RAPIDAPI_KEY")
	req.Header.Add("x-rapidapi-key", apiKey)
	req.Header.Add("x-rapidapi-host", "v3.football.api-sports.io")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	var result map[string]interface{}
	err = json.Unmarshal([]byte(string(body)), &result)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return nil
	}

	response := result["response"].([]interface{})
	return response
}

func noteFixtures(fixtures []interface{}) {
	for _, fixture := range fixtures {
		fixtureMap, ok := fixture.(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting fixture")
			continue
		}

		fixtureID := fixtureMap["fixture"].(map[string]interface{})["id"].(float64)

		exists, err := checkIfRowExists("fixtures", "fixtureId", fixtureID)
		if err != nil {
			fmt.Println("Error checking if row exists:", err)
			continue
		}

		if !exists {
			homeTeamID, ok := fixtureMap["teams"].(map[string]interface{})["home"].(map[string]interface{})["id"].(float64)
			if !ok {
				fmt.Println("Error asserting homeTeamID")
				continue
			}
			awayTeamID, ok := fixtureMap["teams"].(map[string]interface{})["away"].(map[string]interface{})["id"].(float64)
			if !ok {
				fmt.Println("Error asserting awayTeamID")
				continue
			}
			homeTeamScore, ok := fixtureMap["goals"].(map[string]interface{})["home"].(float64)
			if !ok {
				fmt.Println("Error asserting homeTeamScore")
				continue
			}
			awayTeamScore, ok := fixtureMap["goals"].(map[string]interface{})["away"].(float64)
			if !ok {
				fmt.Println("Error asserting awayTeamScore")
				continue
			}

			fmt.Println(fixtureID, homeTeamID, awayTeamID, homeTeamScore, awayTeamScore)

			enterDataIntoDB("fixtures", []string{"fixtureId", "homeTeam", "awayTeam", "homeTeamScore", "awayTeamScore"}, []interface{}{fixtureID, homeTeamID, awayTeamID, homeTeamScore, awayTeamScore})
			enterDataIntoDB("score", []string{"fixtureId", "team", "score"}, []interface{}{fixtureID, homeTeamID, homeTeamScore})
			enterDataIntoDB("score", []string{"fixtureId", "team", "score"}, []interface{}{fixtureID, awayTeamID, awayTeamScore})
		}
	}
}

func main() {
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer closeDB()

	fixtures := getFixturesForYear("2024")
	noteFixtures(fixtures)
}

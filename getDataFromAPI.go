package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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

func PercentagetoFloat(percentage string) float64 {
	percentage = strings.Replace(percentage, "%", "", -1)
	value, err := strconv.ParseFloat(percentage, 64)
	if err != nil {
		fmt.Println("Error converting percentage to float:", err)
		log.Fatal(err)
	}
	return value
}

func filterDataFromFixtures(data []interface{}) (float64, float64, float64, float64, float64, float64) {
	var team1Id, team2Id float64
	var totalShots1, totalShots2 float64
	var ballPossession1, ballPossession2 float64

	for i, teamData := range data {
		teamMap, ok := teamData.(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting teamData to map")
			continue
		}

		team, ok := teamMap["team"].(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting team data")
			continue
		}

		teamId, ok := team["id"].(float64)
		if !ok {
			fmt.Println("Error asserting team ID")
			continue
		}

		statistics, ok := teamMap["statistics"].([]interface{})
		if !ok {
			fmt.Println("Error asserting statistics")
			continue
		}

		for _, stat := range statistics {
			statMap, ok := stat.(map[string]interface{})
			if !ok {
				continue
			}

			switch statMap["type"] {
			case "Total Shots":
				shots, _ := statMap["value"].(float64)
				if i == 0 {
					totalShots1 = shots
				} else {
					totalShots2 = shots
				}
			case "Ball Possession":
				possession, _ := statMap["value"].(string)
				possessionFloat := PercentagetoFloat(possession)
				if i == 0 {
					ballPossession1 = possessionFloat
				} else {
					ballPossession2 = possessionFloat
				}
			}
		}

		if i == 0 {
			team1Id = teamId
		} else {
			team2Id = teamId
		}
	}

	return team1Id, totalShots1, ballPossession1, team2Id, totalShots2, ballPossession2
}

func getAdditionalDataForFixture(fixtureId int) (float64, float64, float64, float64, float64, float64) {
	url := fmt.Sprintf("https://v3.football.api-sports.io/fixtures/statistics?fixture=%d", fixtureId)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	apiKey := os.Getenv("RAPIDAPI_KEY")
	req.Header.Add("x-rapidapi-key", apiKey)
	req.Header.Add("x-rapidapi-host", "v3.football.api-sports.io")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	var result map[string]interface{}
	err = json.Unmarshal([]byte(string(body)), &result)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		log.Fatal(err)
	}

	response := result["response"].([]interface{})

	team1Id, totalShots1, ballPossession1, team2Id, totalShots2, ballPossession2 := filterDataFromFixtures(response)

	if team1Id == 0 || totalShots1 == 0 || ballPossession1 == 0 ||
		team2Id == 0 || totalShots2 == 0 || ballPossession2 == 0 {
		fmt.Println("Warning: One or more values are 0:")
		fmt.Println(fixtureId)
		log.Fatal(response)
	}

	return team1Id, totalShots1, ballPossession1, team2Id, totalShots2, ballPossession2
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
			if fixture, ok := fixtureMap["fixture"].(map[string]interface{}); ok {
				if status, ok := fixture["status"].(map[string]interface{}); ok {
					if short, ok := status["short"].(string); ok && short == "NS" {
						fmt.Println("Game has not started yet")
						continue
					}
				}
			}

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

			team1Id, totalShots1, ballPossession1, team2Id, totalShots2, ballPossession2 := getAdditionalDataForFixture(int(fixtureID))
			enterDataIntoDB("totalShots", []string{"fixtureId", "team", "totalShots"}, []interface{}{fixtureID, team1Id, totalShots1})
			enterDataIntoDB("ballPossession", []string{"fixtureId", "team", "ballPossession"}, []interface{}{fixtureID, team1Id, ballPossession1})
			enterDataIntoDB("totalShots", []string{"fixtureId", "team", "totalShots"}, []interface{}{fixtureID, team2Id, totalShots2})
			enterDataIntoDB("ballPossession", []string{"fixtureId", "team", "ballPossession"}, []interface{}{fixtureID, team2Id, ballPossession2})

			fmt.Println(fixtureID, team1Id, totalShots1, ballPossession1, team2Id, totalShots2, ballPossession2)
			fmt.Println("--------------------------------")

			time.Sleep(7 * time.Second)
		}
	}
}

func main() {
	fmt.Println("Starting program")
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer closeDB()

	fixtures := getFixturesForYear("2024")
	noteFixtures(fixtures)
}

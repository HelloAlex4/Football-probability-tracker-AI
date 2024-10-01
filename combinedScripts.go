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

func getFixtures(year string) []interface{} {
	var url string
	if year == "2024" {
		url = "https://v3.football.api-sports.io/fixtures?season=2024&league=207"
	} else {
		url = "https://v3.football.api-sports.io/fixtures?season=2023&league=207"
	}
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

func writeToDB(response []interface{}) {
	db, err := sql.Open("sqlite3", "./footballTracker.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check if the connection is successful
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Database connection established")

	for _, item := range response {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered from error:", r)
				// Continue to the next iteration if an error occurs
				return
			}
		}()

		fixture, ok := item.(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting fixture")
			continue
		}

		fixtureDetails, ok := fixture["fixture"].(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting fixtureDetails")
			continue
		}

		teamDetails, ok := fixture["teams"].(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting teamDetails")
			continue
		}

		homeTeam, ok := teamDetails["home"].(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting homeTeam")
			continue
		}

		awayTeam, ok := teamDetails["away"].(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting awayTeam")
			continue
		}

		goals, ok := fixture["goals"].(map[string]interface{})
		if !ok {
			fmt.Println("Error asserting goals")
			continue
		}

		homeTeamid, ok := homeTeam["id"].(float64)
		if !ok {
			fmt.Println("Error asserting homeTeamid")
			continue
		}

		awayTeamid, ok := awayTeam["id"].(float64)
		if !ok {
			fmt.Println("Error asserting awayTeamid")
			continue
		}

		fixtureId, ok := fixtureDetails["id"].(float64)
		if !ok {
			fmt.Println("Error asserting fixtureId")
			continue
		}

		timestamp, ok := fixtureDetails["timestamp"].(float64)
		if !ok {
			fmt.Println("Error asserting timestamp")
			continue
		}

		homeTeamGoals, ok := goals["home"].(float64)
		if !ok {
			fmt.Println("Error asserting homeTeamGoals")
			continue
		}

		awayTeamGoals, ok := goals["away"].(float64)
		if !ok {
			fmt.Println("Error asserting awayTeamGoals")
			continue
		}

		fmt.Println("Home Team Goals:", float64(homeTeamGoals))
		fmt.Println("Away Team Goals:", float64(awayTeamGoals))
		fmt.Println("Timestamp:", int(timestamp))
		fmt.Println("Home Team ID:", float64(homeTeamid))
		fmt.Println("Away Team ID:", float64(awayTeamid))
		fmt.Println("Fixture ID:", float64(fixtureId))

		var winnerTeam float64
		if homeTeamGoals > awayTeamGoals {
			winnerTeam = homeTeamid
		} else if homeTeamGoals < awayTeamGoals {
			winnerTeam = awayTeamid
		}

		exists := false
		err := db.QueryRow("SELECT fixtureId FROM fixtures WHERE fixtureId = ?", fixtureId).Scan(&fixtureId)
		if err == nil {
			if err != sql.ErrNoRows {
				exists = true
			} else {
				log.Fatal(err)
			}
		}

		if exists == false && timestamp >= 1704120085 {
			query := `
			INSERT INTO fixtures (fixtureId, home, away, time, winner) VALUES (?, ?, ?, ?, ?);`

			_, err := db.Exec(query, fixtureId, homeTeamid, awayTeamid, timestamp, winnerTeam)
			if err != nil {
				log.Fatal(err)
			}

			query = `
			INSERT INTO goals (fixtureId, team, goals) VALUES (?, ?, ?);`

			_, err = db.Exec(query, fixtureId, homeTeamid, homeTeamGoals)
			if err != nil {
				log.Fatal(err)
			}

			query = `
			INSERT INTO goals (fixtureId, team, goals) VALUES (?, ?, ?);`

			_, err = db.Exec(query, fixtureId, awayTeamid, awayTeamGoals)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func noteData() {
	url := "https://v3.football.api-sports.io/fixtures/statistics?fixture="
	method := "GET"

	db, err := sql.Open("sqlite3", "./footballTracker.db")
	if err != nil {
		fmt.Println("Error opening database:", err)
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT fixtureid FROM fixtures")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fixturesSlice := []string{}

	for rows.Next() {
		var fixture_id string

		err = rows.Scan(&fixture_id)
		if err != nil {
			log.Fatal(err)
		}

		fixturesSlice = append(fixturesSlice, fixture_id)
	}

	for _, fixtureId := range fixturesSlice {
		exists := false
		err := db.QueryRow("SELECT fixture FROM passes WHERE fixture = ?", fixtureId).Scan(&fixtureId)
		if err == nil {
			if err != sql.ErrNoRows {
				exists = true
			} else {
				log.Fatal(err)
			}
		}

		fmt.Println(exists)

		if exists == false {
			fullURL := url + fixtureId

			client := &http.Client{}
			req, err := http.NewRequest(method, fullURL, nil)
			if err != nil {
				fmt.Println(err)
				return
			}

			req.Header.Add("x-rapidapi-key", "7ab746690d8fc853131cac5708917adb")
			req.Header.Add("x-rapidapi-host", "v3.football.api-sports.io")

			res, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer res.Body.Close()

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				fmt.Println(err)
				return
			}

			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			if err != nil {
				fmt.Println("Error unmarshaling JSON:", err)
				return
			}

			// Safely access response to avoid nil panic
			response, ok := result["response"].([]interface{})
			if !ok {
				fmt.Println("Error: response is not of type []interface{}")
				continue
			}

			for _, item := range response {
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					fmt.Println("Error: item is not of type map[string]interface{}")
					continue
				}

				// Correctly access the "team" map and retrieve "id"
				team, ok := itemMap["team"].(map[string]interface{})
				if !ok {
					fmt.Println("Error: team is not of type map[string]interface{}")
					continue
				}
				teamID := int(team["id"].(float64))

				// Correctly assert "statistics" as a slice of interfaces
				statistics, ok := itemMap["statistics"].([]interface{})
				if !ok {
					fmt.Println("Error: statistics is not of type []interface{}")
					continue
				}

				// Initialize variables outside the loop
				var totalShots, ballPossession, passes int

				for _, stat := range statistics {
					statMap, ok := stat.(map[string]interface{})
					if !ok {
						fmt.Println("Error: stat is not of type map[string]interface{}")
						continue
					}

					// Extract type and value from statistics
					data := statMap["type"].(string)

					// Check for Total Shots
					if data == "Total Shots" {
						fmt.Println("TOTAL SHOTS")
						if value, ok := statMap["value"].(float64); ok {
							totalShots = int(value)
						}
					}

					// Check for Ball Possession
					if data == "Ball Possession" {
						fmt.Println("BALL POSSESSION")
						if value, ok := statMap["value"].(string); ok {
							cleanStr := strings.TrimSuffix(value, "%")
							ballPossession, err = strconv.Atoi(cleanStr)
							if err != nil {
								log.Fatal(err)
								return
							}
						}
					}

					// Check for Passes %
					if data == "Passes %" {
						fmt.Println("PASSES")
						if value, ok := statMap["value"].(string); ok {
							cleanStr := strings.TrimSuffix(value, "%")
							passes, err = strconv.Atoi(cleanStr)
							if err != nil {
								log.Fatal(err)
								return
							}
						}
					}
				}

				fmt.Println("Team ID:", teamID)
				fmt.Println("Total Shots:", totalShots)
				fmt.Println("Ball Possession:", ballPossession)
				fmt.Println("Passes:", passes)

				// Execute INSERT statements
				_, err = db.Exec("INSERT INTO totalshots (fixtureId, team, shots) VALUES (?, ?, ?);", fixtureId, teamID, totalShots)
				if err != nil {
					log.Fatal(err)
				}

				_, err = db.Exec("INSERT INTO ballpossession (fixtureId, team, possession) VALUES (?, ?, ?);", fixtureId, teamID, ballPossession)
				if err != nil {
					log.Fatal(err)
				}

				_, err = db.Exec("INSERT INTO passes (fixtureId, team, passes) VALUES (?, ?, ?);", fixtureId, teamID, passes)
				if err != nil {
					log.Fatal(err)
				}

				time.Sleep(10 * time.Second)
			}
		}
	}
}

var db *sql.DB

func noteFixture() {
	response := getFixtures("2023")
	writeToDB(response)

	response = getFixtures("2024")
	writeToDB(response)
}

func main() {
	noteFixture()
	noteData()
}

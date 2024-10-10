package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./FootballTracker.db?_timeout=10000&_busy_timeout=10000")
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	return nil
}

func closeDB() {
	db.Close()
}

func getEloForTeam(teamID int) (float64, error) {
	query := "SELECT * from elo where team = ?"
	row := db.QueryRow(query, teamID)
	var team float64
	var goalElo float64
	var winnerElo float64
	var totalShotsElo float64
	var ballPossessionElo float64

	err := row.Scan(&team, &goalElo, &winnerElo, &totalShotsElo, &ballPossessionElo)
	if err != nil {
		return 0, fmt.Errorf("failed to get elo for team: %v", err)
	}

	averagedElo := goalElo*0.4 + winnerElo*0.3 + totalShotsElo*0.15 + ballPossessionElo*0.15
	return averagedElo, nil
}

func calcChancesFromElo(team1Elo float64, team2Elo float64) (float64, float64) {
	team1Chances := 1 / (1 + math.Pow(10, (team2Elo-team1Elo)/400))
	team2Chances := 1 / (1 + math.Pow(10, (team1Elo-team2Elo)/400))
	return team1Chances, team2Chances
}

func calculateChances(team1ID int, team2ID int) (float64, float64) {
	team1Elo, err := getEloForTeam(team1ID)
	if err != nil {
		log.Fatalf("Failed to get elo for team: %v", err)
	}
	team2Elo, err := getEloForTeam(team2ID)
	if err != nil {
		log.Fatalf("Failed to get elo for team: %v", err)
	}

	return calcChancesFromElo(team1Elo, team2Elo)
}

func fullProcess(team1 string, team2 string) {
	team1ID, found1 := reverseTeamData[strings.ToLower(team1)]
	team2ID, found2 := reverseTeamData[strings.ToLower(team2)]

	if !found1 || !found2 {
		log.Fatalf("Team not found in the map")
	}

	team1Chances, team2Chances := calculateChances(team1ID, team2ID)

	// Round to 2 decimal places
	fmt.Printf("%s: %s%%\n", team1, fmt.Sprintf("%.2f", team1Chances*100))
	fmt.Printf("%s: %s%%\n", team2, fmt.Sprintf("%.2f", team2Chances*100))
	fmt.Printf("-----------------------------------\n\n")
}

var teamData = map[int]string{
	606:  "lugano",
	1013: "grasshoppers",
	6653: "yverdon",
	783:  "zurich",
	2180: "winterthur",
	1011: "st gallen",
	565:  "young boys",
	630:  "sion",
	644:  "luzern",
	2184: "servette",
	1014: "lausanne",
	551:  "basel",
}

var reverseTeamData map[string]int

func main() {
	err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer closeDB()

	// Create a reverse lookup map
	reverseTeamData = make(map[string]int)
	for id, name := range teamData {
		reverseTeamData[strings.ToLower(name)] = id
	}

	fmt.Println("")

	fullProcess("grasshoppers", "zurich")
	fullProcess("luzern", "young boys")
	fullProcess("servette", "sion")
	fullProcess("basel", "st gallen")
	fullProcess("yverdon", "lugano")
	fullProcess("lausanne", "winterthur")
}

package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
)

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

func normalizeScore(max float64, min float64, score float64) float64 {
	return (score - min) / (max - min)
}

func getCurrentEloFromDB(teamId string) float64 {
	db := getDB()

	query := "SELECT goalElo FROM elo_rankings WHERE team_id = ?"
	row := db.QueryRow(query, teamId)

	var elo float64
	err := row.Scan(&elo)
	if err != nil {
		if err == sql.ErrNoRows {
			// No existing Elo rating found, insert default value
			_, err := db.Exec("INSERT INTO goalElo (team_id, elo) VALUES (?, ?)", teamId, 1000)
			if err != nil {
				log.Printf("Error inserting default Elo: %v", err)
			}
			return 1000
		}
		log.Printf("Error querying Elo: %v", err)
		return 1000
	}

	return elo
}

func calcExpectedElo(opponentElo float64, TeamElo float64) float64 {
	newElo := 1 / (1 + math.Pow(10, (opponentElo-TeamElo)/400))

	return newElo
}

func updateEloForScores(TeamElo float64, expectedScore float64, score float64, kFactor float64) float64 {
	updatedElo := TeamElo + kFactor*(score-expectedScore)

	return updatedElo
}

func getMaxMinScore() (float64, float64) {
	db := getDB()

	var maxScore float64
	var minScore float64

	query := "SELECT MAX(score), MIN(score) FROM score"
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}

	for rows.Next() {
		err = rows.Scan(&maxScore)
		if err != nil {
			log.Fatal(err)
		}
	}

	return maxScore, minScore
}

func calcEloForScores() {
	db := getDB()

	query := "SELECT * FROM Fixtures"

	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}

	for rows.Next() {
		var homeTeamId int
		var awayTeamId int
		var homeTeamScore int
		var awayTeamScore int

		err = rows.Scan(&homeTeamId, &awayTeamId, &homeTeamScore, &awayTeamScore)
		if err != nil {
			log.Fatal(err)
		}

		maxScore, minScore := getMaxMinScore()

		normalizedHomeTeamScore := normalizeScore(maxScore, minScore, float64(homeTeamScore))
		normalizedAwayTeamScore := normalizeScore(maxScore, minScore, float64(awayTeamScore))

		homeTeamIdString := strconv.Itoa(homeTeamId)
		awayTeamIdString := strconv.Itoa(awayTeamId)

		homeTeamElo := getCurrentEloFromDB(homeTeamIdString)
		awayTeamElo := getCurrentEloFromDB(awayTeamIdString)

		expectedHomeTeamScore := calcExpectedElo(awayTeamElo, homeTeamElo)
		expectedAwayTeamScore := calcExpectedElo(homeTeamElo, awayTeamElo)

		updatedHomeTeamElo := updateEloForScores(homeTeamElo, expectedHomeTeamScore, normalizedHomeTeamScore, 20)
		updatedAwayTeamElo := updateEloForScores(awayTeamElo, expectedAwayTeamScore, normalizedAwayTeamScore, 20)

		enterDataIntoDB("elo_rankings", []string{"team_id", "elo"}, []interface{}{homeTeamIdString, updatedHomeTeamElo})
		enterDataIntoDB("elo_rankings", []string{"team_id", "elo"}, []interface{}{awayTeamIdString, updatedAwayTeamElo})
	}
}

func main() {
	initDB()
	defer closeDB()

	calcEloForScores()
}

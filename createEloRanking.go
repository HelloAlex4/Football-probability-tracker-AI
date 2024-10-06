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

func updateEloForTeam(teamId int, elo float64) {
	query := "UPDATE elo SET goalElo = ? WHERE team = ?"
	_, err := db.Exec(query, elo, teamId)
	if err != nil {
		log.Printf("Error updating Elo: %v", err)
	}
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
	_, err := db.Exec(query, data...)
	if err != nil {
		return fmt.Errorf("failed to insert data: %v", err)
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

func getCurrentEloFromDB(teamId int) float64 {
	query := "SELECT goalElo FROM elo WHERE team = ?"
	row := db.QueryRow(query, teamId)

	var elo float64
	err := row.Scan(&elo)
	if err != nil {
		if err == sql.ErrNoRows {
			// No existing Elo rating found, insert default value
			_, err := db.Exec("INSERT INTO elo (team, goalelo) VALUES (?, ?)", teamId, 1000)
			fmt.Println("Inserting default Elo")
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
	var maxScore float64
	var minScore float64

	query := "SELECT MAX(score), MIN(score) FROM score"
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&maxScore, &minScore)
		if err != nil {
			log.Fatal(err)
		}
	}

	return maxScore, minScore
}

func calcEloForScores() {
	query := "SELECT * FROM fixtures"

	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var fixtureId int
		var homeTeamId int
		var awayTeamId int
		var homeTeamScore int
		var awayTeamScore int

		err = rows.Scan(&fixtureId, &homeTeamId, &awayTeamId, &homeTeamScore, &awayTeamScore)
		if err != nil {
			log.Fatal(err)
		}

		maxScore, minScore := getMaxMinScore()

		homeTeamElo := getCurrentEloFromDB(homeTeamId)
		awayTeamElo := getCurrentEloFromDB(awayTeamId)

		normalizedHomeTeamScore := normalizeScore(maxScore, minScore, float64(homeTeamScore))
		normalizedAwayTeamScore := normalizeScore(maxScore, minScore, float64(awayTeamScore))

		fmt.Println(normalizedHomeTeamScore, normalizedAwayTeamScore)

		expectedHomeTeamScore := calcExpectedElo(awayTeamElo, homeTeamElo)
		expectedAwayTeamScore := calcExpectedElo(homeTeamElo, awayTeamElo)

		updatedHomeTeamElo := updateEloForScores(homeTeamElo, expectedHomeTeamScore, normalizedHomeTeamScore, 25)
		updatedAwayTeamElo := updateEloForScores(awayTeamElo, expectedAwayTeamScore, normalizedAwayTeamScore, 25)

		updateEloForTeam(homeTeamId, updatedHomeTeamElo)
		updateEloForTeam(awayTeamId, updatedAwayTeamElo)
	}

}

//1: get elos
//2: get values
//3: normalize values
//4: calc expected elos
//5: calac updated elos
//6: update elos
//scoreElo, winnerElo, ballPossessionElo, shotsOnTargetElo

func main() {
	err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer closeDB()

	calcEloForScores()
}

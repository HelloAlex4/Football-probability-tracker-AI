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

func getCurrentEloFromDB(elotype string, teamId int) float64 {
	query := fmt.Sprintf("SELECT %sElo FROM elo WHERE team = ?", elotype)
	row := db.QueryRow(query, teamId)

	var elo float64
	err := row.Scan(&elo)
	if err != nil {
		if err == sql.ErrNoRows {
			// No existing Elo rating found, insert default value
			_, err := db.Exec(fmt.Sprintf("INSERT INTO elo (team, %sElo) VALUES (?, ?)", elotype), teamId, 1000)
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

	query := "SELECT MAX(score), MIN(score) FROM fixtures"
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

func getBallPossessionScore(homeTeam int, awayTeam int, fixtureId int) (float64, float64, float64, float64) {
	homeTeamQuery := fmt.Sprintf("SELECT ballPossession FROM ballPossession WHERE fixtureId = %d AND teamId = %d", fixtureId, homeTeam)
	awayTeamQuery := fmt.Sprintf("SELECT ballPossession FROM ballPossession WHERE fixtureId = %d AND teamId = %d", fixtureId, awayTeam)

	minMaxQuery := "SELECT MIN(ballPossession), MAX(ballPossession) FROM ballPossession"

	var minScore float64
	var maxScore float64

	var homeTeamBallPossession float64
	var awayTeamBallPossession float64

	row := db.QueryRow(homeTeamQuery)
	row.Scan(&homeTeamBallPossession)

	row = db.QueryRow(awayTeamQuery)
	row.Scan(&awayTeamBallPossession)

	row = db.QueryRow(minMaxQuery)
	row.Scan(&minScore, &maxScore)

	return homeTeamBallPossession, awayTeamBallPossession, maxScore, minScore
}

func getShotsOnTargetScore(homeTeam int, awayTeam int, fixtureId int) (float64, float64, float64, float64) {
	homeTeamQuery := fmt.Sprintf("SELECT totalShots FROM totalShots WHERE fixtureId = %d AND teamId = %d", fixtureId, homeTeam)
	awayTeamQuery := fmt.Sprintf("SELECT totalShots FROM totalShots WHERE fixtureId = %d AND teamId = %d", fixtureId, awayTeam)

	minMaxQuery := "SELECT MIN(totalShots), MAX(totalShots) FROM totalShots"

	var minScore float64
	var maxScore float64

	var homeTeamShotsOnTarget float64
	var awayTeamShotsOnTarget float64

	row := db.QueryRow(homeTeamQuery)
	row.Scan(&homeTeamShotsOnTarget)

	row = db.QueryRow(awayTeamQuery)
	row.Scan(&awayTeamShotsOnTarget)

	row = db.QueryRow(minMaxQuery)
	row.Scan(&minScore, &maxScore)

	return homeTeamShotsOnTarget, awayTeamShotsOnTarget, maxScore, minScore
}

func getWinnerScore(homeTeamScore int, awayTeamScore int) (float64, float64) {
	if homeTeamScore > awayTeamScore {
		return 1, 0
	}
	return 0, 1
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

		//score get elos
		homeTeamGoalElo := getCurrentEloFromDB("goalElo", homeTeamId)
		awayTeamGoalElo := getCurrentEloFromDB("goalElo", awayTeamId)

		//ball possession gel elo
		homeTeamBallPossessionElo := getCurrentEloFromDB("ballPossessionElo", homeTeamId)
		awayTeamBallPossessionElo := getCurrentEloFromDB("ballPossessionElo", awayTeamId)

		//shots on target get elo
		homeTeamShotsOnTargetElo := getCurrentEloFromDB("totalShotsElo", homeTeamId)
		awayTeamShotsOnTargetElo := getCurrentEloFromDB("totalShotsElo", awayTeamId)

		//winner get elo
		homeTeamWinnerElo := getCurrentEloFromDB("winnerElo", homeTeamId)
		awayTeamWinnerElo := getCurrentEloFromDB("winnerElo", awayTeamId)

		//score get score
		//already defined since it is in the fixtures table

		//ball possession get score
		homeTeamBallPossession, awayTeamBallPossession, ballPossessionMaxScore, ballPossessionMinScore := getBallPossessionScore(homeTeamId, awayTeamId, fixtureId)

		//shots on target get score
		homeTeamShotsOnTarget, awayTeamShotsOnTarget, shotsOnTargetMaxScore, shotsOnTargetMinScore := getShotsOnTargetScore(homeTeamId, awayTeamId, fixtureId)

		//winner get score
		//skipped since it is computed with the score values

		//score
		minScore, maxScore := getMaxMinScore()
		normalizedHomeTeamScore := normalizeScore(maxScore, minScore, float64(homeTeamScore))
		normalizedAwayTeamScore := normalizeScore(maxScore, minScore, float64(awayTeamScore))

		//ball possession
		normalizedHomeTeamBallPossession := normalizeScore(ballPossessionMaxScore, ballPossessionMinScore, homeTeamBallPossession)
		normalizedAwayTeamBallPossession := normalizeScore(ballPossessionMaxScore, ballPossessionMinScore, awayTeamBallPossession)

		//shots on target
		normalizedHomeTeamShotsOnTarget := normalizeScore(shotsOnTargetMaxScore, shotsOnTargetMinScore, homeTeamShotsOnTarget)
		normalizedAwayTeamShotsOnTarget := normalizeScore(shotsOnTargetMaxScore, shotsOnTargetMinScore, awayTeamShotsOnTarget)

		//winner
		//get the normalized value instantly since it is always 1 or 0
		homeTeamWinnerValue, awayTeamWinnerValue := getWinnerScore(homeTeamScore, awayTeamScore)

		//score
		expectedHomeTeamScore := calcExpectedElo(awayTeamGoalElo, homeTeamGoalElo)
		expectedAwayTeamScore := calcExpectedElo(homeTeamGoalElo, awayTeamGoalElo)

		//winner
		expectedHomeTeamWinner := calcExpectedElo(awayTeamWinnerElo, homeTeamWinnerElo)
		expectedAwayTeamWinner := calcExpectedElo(homeTeamWinnerElo, awayTeamWinnerElo)

		//ball possession
		expectedHomeTeamBallPossession := calcExpectedElo(awayTeamBallPossessionElo, homeTeamBallPossessionElo)
		expectedAwayTeamBallPossession := calcExpectedElo(homeTeamBallPossessionElo, awayTeamBallPossessionElo)

		//shots on target
		expectedHomeTeamShotsOnTarget := calcExpectedElo(awayTeamShotsOnTargetElo, homeTeamShotsOnTargetElo)
		expectedAwayTeamShotsOnTarget := calcExpectedElo(homeTeamShotsOnTargetElo, awayTeamShotsOnTargetElo)

		//score
		updatedHomeTeamScoreElo := updateEloForScores(homeTeamGoalElo, expectedHomeTeamScore, normalizedHomeTeamScore, 25)
		updatedAwayTeamScoreElo := updateEloForScores(awayTeamGoalElo, expectedAwayTeamScore, normalizedAwayTeamScore, 25)

		//ball possession
		updatedHomeTeamBallPossessionElo := updateEloForScores(homeTeamBallPossessionElo, expectedHomeTeamBallPossession, normalizedHomeTeamBallPossession, 25)
		updatedAwayTeamBallPossessionElo := updateEloForScores(awayTeamBallPossessionElo, expectedAwayTeamBallPossession, normalizedAwayTeamBallPossession, 25)

		//shots on target
		updatedHomeTeamShotsOnTargetElo := updateEloForScores(homeTeamShotsOnTargetElo, expectedHomeTeamShotsOnTarget, normalizedHomeTeamShotsOnTarget, 25)
		updatedAwayTeamShotsOnTargetElo := updateEloForScores(awayTeamShotsOnTargetElo, expectedAwayTeamShotsOnTarget, normalizedAwayTeamShotsOnTarget, 25)

		//winner
		updatedHomeTeamWinnerElo := updateEloForScores(homeTeamWinnerElo, expectedHomeTeamWinner, homeTeamWinnerValue, 25)
		updatedAwayTeamWinnerElo := updateEloForScores(awayTeamWinnerElo, expectedAwayTeamWinner, awayTeamWinnerValue, 25)

		//score
		updateEloForTeam(homeTeamId, updatedHomeTeamScoreElo)
		updateEloForTeam(awayTeamId, updatedAwayTeamScoreElo)

		//ball possession
		updateEloForTeam(homeTeamId, updatedHomeTeamBallPossessionElo)
		updateEloForTeam(awayTeamId, updatedAwayTeamBallPossessionElo)

		//shots on target
		updateEloForTeam(homeTeamId, updatedHomeTeamShotsOnTargetElo)
		updateEloForTeam(awayTeamId, updatedAwayTeamShotsOnTargetElo)

		//winner
		updateEloForTeam(homeTeamId, updatedHomeTeamWinnerElo)
		updateEloForTeam(awayTeamId, updatedAwayTeamWinnerElo)
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

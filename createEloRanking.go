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

func updateEloForTeam(elotype string, teamId int, elo float64) {
	query := fmt.Sprintf("UPDATE elo SET %s = ? WHERE team = ?", elotype)
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
	query := fmt.Sprintf("SELECT %s FROM elo WHERE team = ?", elotype)
	row := db.QueryRow(query, teamId)

	var elo sql.NullFloat64
	err := row.Scan(&elo)
	if err != nil {
		if err == sql.ErrNoRows {
			// No existing Elo rating found, insert default value
			_, err := db.Exec(fmt.Sprintf("INSERT INTO elo (team, %s) VALUES (?, ?)", elotype), teamId, 1000)
			fmt.Println("Inserting default Elo")
			if err != nil {
				log.Printf("Error inserting default Elo: %v", err)
			}
			return 1000
		}
		log.Printf("Error querying Elo: %v", err)
		return 1000
	}

	if elo.Valid {
		return elo.Float64
	}
	return 1000 // Return default value if NULL
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

	query := `
		SELECT 
			MAX(CASE WHEN homeTeamScore > awayTeamScore THEN homeTeamScore ELSE awayTeamScore END) as max_score,
			MIN(CASE WHEN homeTeamScore < awayTeamScore THEN homeTeamScore ELSE awayTeamScore END) as min_score
		FROM fixtures
	`
	row := db.QueryRow(query)
	err := row.Scan(&maxScore, &minScore)
	if err != nil {
		log.Printf("Error querying max and min scores: %v", err)
		return 0, 0
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
		updateEloForTeam("goalElo", homeTeamId, updatedHomeTeamScoreElo)
		updateEloForTeam("goalElo", awayTeamId, updatedAwayTeamScoreElo)

		//ball possession
		updateEloForTeam("ballPossessionElo", homeTeamId, updatedHomeTeamBallPossessionElo)
		updateEloForTeam("ballPossessionElo", awayTeamId, updatedAwayTeamBallPossessionElo)

		//shots on target
		updateEloForTeam("totalShotsElo", homeTeamId, updatedHomeTeamShotsOnTargetElo)
		updateEloForTeam("totalShotsElo", awayTeamId, updatedAwayTeamShotsOnTargetElo)

		//winner
		updateEloForTeam("winnerElo", homeTeamId, updatedHomeTeamWinnerElo)
		updateEloForTeam("winnerElo", awayTeamId, updatedAwayTeamWinnerElo)
	}
}

//1: get elos
//2: get values
//3: normalize values
//4: calc expected elos
//5: calac updated elos
//6: update elos
//scoreElo, winnerElo, ballPossessionElo, shotsOnTargetElo

func normalizeEloValues() {
	query := "SELECT MAX(goalElo), MAX(winnerElo), MAX(ballPossessionElo), MAX(totalShotsElo), MIN(goalElo), MIN(winnerElo), MIN(ballPossessionElo), MIN(totalShotsElo) FROM elo"

	var maxGoalElo float64
	var maxWinnerElo float64
	var maxBallPossessionElo float64
	var maxShotsOnTargetElo float64
	var minGoalElo float64
	var minWinnerElo float64
	var minBallPossessionElo float64
	var minShotsOnTargetElo float64

	row := db.QueryRow(query)
	row.Scan(&maxGoalElo, &maxWinnerElo, &maxBallPossessionElo, &maxShotsOnTargetElo, &minGoalElo, &minWinnerElo, &minBallPossessionElo, &minShotsOnTargetElo)

	query = "SELECT * FROM elo"
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var teamId int
		var goalElo float64
		var winnerElo float64
		var ballPossessionElo float64
		var totalShotsElo float64

		err = rows.Scan(&teamId, &goalElo, &winnerElo, &totalShotsElo, &ballPossessionElo)
		if err != nil {
			log.Fatal(err)
		}

		normalizedGoalElo := normalizeScore(maxGoalElo, minGoalElo, goalElo)
		normalizedWinnerElo := normalizeScore(maxWinnerElo, minWinnerElo, winnerElo)
		normalizedBallPossessionElo := normalizeScore(maxBallPossessionElo, minBallPossessionElo, ballPossessionElo)
		normalizedTotalShotsElo := normalizeScore(maxShotsOnTargetElo, minShotsOnTargetElo, totalShotsElo)

		normalizedGoalElo = 1000 + normalizedGoalElo*1000
		normalizedWinnerElo = 1000 + normalizedWinnerElo*1000
		normalizedBallPossessionElo = 1000 + normalizedBallPossessionElo*1000
		normalizedTotalShotsElo = 1000 + normalizedTotalShotsElo*1000

		updateEloForTeam("goalElo", teamId, normalizedGoalElo)
		updateEloForTeam("winnerElo", teamId, normalizedWinnerElo)
		updateEloForTeam("ballPossessionElo", teamId, normalizedBallPossessionElo)
		updateEloForTeam("totalShotsElo", teamId, normalizedTotalShotsElo)
	}
}

func main() {
	err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer closeDB()

	query := "DELETE FROM elo"
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("Error deleting data: %v", err)
	}

	calcEloForScores()
	normalizeEloValues()
}

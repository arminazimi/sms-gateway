package db

import (
	"os"
	"strings"
)

func MigrateFromFile(database *DB, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	sqlText := string(content)
	sqlText = strings.ReplaceAll(sqlText, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ")

	statements := strings.Split(sqlText, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "#") || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := database.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

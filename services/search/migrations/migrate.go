package migrations

import (
	"embed"
	"fmt"
	"strings"

	"tachyon-messenger/shared/database"
)

//go:embed *.sql
var migrationFiles embed.FS

// RunMigrations runs all database migrations for the search service
func RunMigrations(db *database.DB) error {
	fmt.Println("Running search service SQL migrations...")

	sqlBytes, err := migrationFiles.ReadFile("001_initial_search_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Split by double newline + statement boundary to handle $$ blocks properly
	sqlContent := string(sqlBytes)

	// Execute the entire migration as individual statements
	// We need special handling for $$ blocks (trigger functions)
	statements := splitSQLStatements(sqlContent)

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		if err := db.Exec(stmt).Error; err != nil {
			// Skip "already exists" errors for idempotent statements
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			return fmt.Errorf("migration failed on statement: %s\nerror: %w", truncateStr(stmt, 100), err)
		}
	}

	fmt.Println("Search service migrations completed successfully")
	return nil
}

// splitSQLStatements splits SQL content into individual statements,
// correctly handling $$ delimited function bodies
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inDollarBlock := false

	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip pure comment lines at statement start
		if current.Len() == 0 && strings.HasPrefix(trimmed, "--") {
			continue
		}

		// Track $$ blocks
		dollarCount := strings.Count(line, "$$")
		if dollarCount%2 != 0 {
			inDollarBlock = !inDollarBlock
		}

		current.WriteString(line)
		current.WriteString("\n")

		// If we're outside a $$ block and the line ends with ;, it's the end of a statement
		if !inDollarBlock && strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}

	// Add any remaining content
	if remaining := strings.TrimSpace(current.String()); remaining != "" {
		statements = append(statements, remaining)
	}

	return statements
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

package httpkit

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func IsUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 42P01 = undefined_table
		return pgErr.Code == "42P01"
	}
	return false
}

// IsUniqueViolation returns true if the error is a PostgreSQL unique constraint violation.
// 23505 = unique_violation
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

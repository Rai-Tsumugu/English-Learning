package repo

import (
	"context"
	"database/sql"
	"time"
)

// User mirrors the users row.
type User struct {
	ID        int64
	CEFRSelf  sql.NullString
	Theta     float64
	SEM       float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Users is the users repository.
type Users struct{ db *sql.DB }

// NewUsers constructs a Users repository.
func NewUsers(db *sql.DB) *Users { return &Users{db: db} }

// Create inserts a new user and returns the assigned ID.
func (r *Users) Create(ctx context.Context, cefrSelf string, theta, sem float64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users(cefr_self, theta, sem) VALUES (?, ?, ?)`,
		nullableString(cefrSelf), theta, sem)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetByID fetches a user by primary key.
func (r *Users) GetByID(ctx context.Context, id int64) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, cefr_self, theta, sem, created_at, updated_at FROM users WHERE id=?`, id)
	u := &User{}
	if err := row.Scan(&u.ID, &u.CEFRSelf, &u.Theta, &u.SEM, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateTheta updates the IRT ability estimate (theta) and standard error
// of measurement (sem) for the user.
func (r *Users) UpdateTheta(ctx context.Context, id int64, theta, sem float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET theta=?, sem=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		theta, sem, id)
	return err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

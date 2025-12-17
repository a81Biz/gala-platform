package repositories

import (
	"context"
	"errors"

	"gala/internal/httpkit"
	"gala/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTemplateNotFound = errors.New("template not found")
var ErrTemplateNameExists = errors.New("template name already exists")

type TemplateRepository struct {
	db *pgxpool.Pool
}

func NewTemplateRepository(db *pgxpool.Pool) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Create(ctx context.Context, t *models.Template) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO templates (id, name, description, definition_json)
		VALUES ($1,$2,$3,$4)
		RETURNING created_at
	`, t.ID, t.Name, t.Description, t.Definition).Scan(&t.CreatedAt)

	if err != nil {
		if httpkit.IsUniqueViolation(err) {
			return ErrTemplateNameExists
		}
		return err
	}
	return nil
}

func (r *TemplateRepository) List(ctx context.Context) ([]models.Template, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, created_at
		FROM templates
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Template
	for rows.Next() {
		var t models.Template
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *TemplateRepository) Get(ctx context.Context, id string) (*models.Template, error) {
	var t models.Template
	err := r.db.QueryRow(ctx, `
		SELECT id, name, description, definition_json, created_at, deleted_at
		FROM templates
		WHERE id=$1
	`, id).Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&t.Definition,
		&t.CreatedAt,
		&t.DeletedAt,
	)
	if err != nil {
		return nil, ErrTemplateNotFound
	}
	return &t, nil
}

func (r *TemplateRepository) Delete(ctx context.Context, id string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE templates
		SET deleted_at=now()
		WHERE id=$1 AND deleted_at IS NULL
	`)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

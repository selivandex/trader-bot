package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Repository handles database operations for risk management
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new risk repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// RiskEvent represents a risk event record
type RiskEvent struct {
	ID          int64                  `db:"id"`
	UserID      int64                  `db:"user_id"`
	EventType   string                 `db:"event_type"`
	Description string                 `db:"description"`
	Data        map[string]interface{} `db:"data"`
	CreatedAt   time.Time              `db:"created_at"`
}

// LogRiskEvent logs a risk event to database
func (r *Repository) LogRiskEvent(ctx context.Context, userID int64, eventType, description string, data map[string]interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	query := `
		INSERT INTO risk_events (user_id, event_type, description, data, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	var id int64
	err = r.db.QueryRowContext(ctx, query, userID, eventType, description, dataJSON, time.Now()).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to log risk event: %w", err)
	}

	return nil
}

// GetRecentRiskEvents retrieves recent risk events for a user
func (r *Repository) GetRecentRiskEvents(ctx context.Context, userID int64, limit int) ([]RiskEvent, error) {
	query := `
		SELECT id, user_id, event_type, description, data, created_at
		FROM risk_events
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query risk events: %w", err)
	}
	defer rows.Close()

	events := make([]RiskEvent, 0)
	for rows.Next() {
		var event RiskEvent
		var dataJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.EventType,
			&event.Description,
			&dataJSON,
			&event.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Unmarshal data
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &event.Data); err == nil {
				events = append(events, event)
			}
		}
	}

	return events, nil
}

// GetRiskEventsByType retrieves risk events by type for a user
func (r *Repository) GetRiskEventsByType(ctx context.Context, userID int64, eventType string, since time.Time) ([]RiskEvent, error) {
	query := `
		SELECT id, user_id, event_type, description, data, created_at
		FROM risk_events
		WHERE user_id = $1 AND event_type = $2 AND created_at >= $3
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, eventType, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query risk events by type: %w", err)
	}
	defer rows.Close()

	events := make([]RiskEvent, 0)
	for rows.Next() {
		var event RiskEvent
		var dataJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.EventType,
			&event.Description,
			&dataJSON,
			&event.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Unmarshal data
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &event.Data); err == nil {
				events = append(events, event)
			}
		}
	}

	return events, nil
}

// CountRiskEventsByType counts risk events by type for a user within a time period
func (r *Repository) CountRiskEventsByType(ctx context.Context, userID int64, eventType string, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM risk_events
		WHERE user_id = $1 AND event_type = $2 AND created_at >= $3
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID, eventType, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count risk events: %w", err)
	}

	return count, nil
}

// DeleteOldRiskEvents deletes risk events older than specified duration
func (r *Repository) DeleteOldRiskEvents(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM risk_events
		WHERE created_at < $1
	`

	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old risk events: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}


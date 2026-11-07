package httputil

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
)

const (
	defaultPageLimit = 25
	maxPageLimit     = 100
)

type PageRequest struct {
	Limit  int
	Cursor string
}

type PageResponse[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type cursorPayload struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func ParsePageRequest(r *http.Request) (PageRequest, error) {
	query := r.URL.Query()

	limit, err := parseLimit(query.Get("limit"))
	if err != nil {
		return PageRequest{}, err
	}

	cursor := query.Get("cursor")
	if cursor != "" {
		if _, _, err := DecodeCursor(cursor); err != nil {
			return PageRequest{}, err
		}
	}

	return PageRequest{Limit: limit, Cursor: cursor}, nil
}

func EncodeCursor(id uuid.UUID, createdAt time.Time) string {
	raw, _ := json.Marshal(cursorPayload{ID: id, CreatedAt: createdAt.UTC()})

	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeCursor(cursor string) (uuid.UUID, time.Time, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return uuid.Nil, time.Time{}, invalidCursor()
	}

	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.ID == uuid.Nil {
		return uuid.Nil, time.Time{}, invalidCursor()
	}

	return payload.ID, payload.CreatedAt, nil
}

func NewPageResponse[T any](items []T, limit int, cursorFor func(T) string) PageResponse[T] {
	if items == nil {
		items = []T{}
	}

	if len(items) <= limit {
		return PageResponse[T]{Items: items}
	}

	items = items[:limit]
	next := cursorFor(items[limit-1])

	return PageResponse[T]{Items: items, NextCursor: &next, HasMore: true}
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultPageLimit, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxPageLimit {
		return 0, apperror.Validation("invalid pagination parameters", map[string]string{
			"limit": "must be an integer between 1 and 100",
		})
	}

	return limit, nil
}

func invalidCursor() error {
	return apperror.Validation("invalid pagination parameters", map[string]string{
		"cursor": "is not a valid pagination cursor",
	})
}

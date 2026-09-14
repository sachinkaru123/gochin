package orm

import "errors"

// Pagination defaults. HardRowCap is the structural backstop: it bounds any
// unpaginated Get(), so a forgotten LIMIT on a large table cannot exhaust
// memory.
var (
	DefaultPerPage = 25
	MaxPerPage     = 100
	HardRowCap     = 10_000
)

// ErrTooManyRows is returned when a query would return more than HardRowCap
// rows. Truncating silently would give wrong answers, so this fails loudly.
var ErrTooManyRows = errors.New("orm: query exceeded the row cap; use Paginate or Limit")

// Page is one page of results plus the counts needed to render pagination.
type Page[T any] struct {
	Data       []*T   `json:"data"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	Total      int64  `json:"total"`
	TotalPages int    `json:"total_pages"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// ClampPagination bounds a requested page and page size to sane values.
func ClampPagination(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}
	return page, perPage
}

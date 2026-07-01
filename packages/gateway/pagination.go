package gateway

import (
	"fmt"
	"net/http"
	"strconv"
)

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

// ParsePagination extracts and validates "page" and "per_page" query parameters
// from r. Both default to sensible values when absent. per_page is capped at 100.
func ParsePagination(r *http.Request) (page, perPage int, err error) {
	page = defaultPage
	perPage = defaultPerPage

	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return 0, 0, &APIError{
				Code:    ErrCodeBadRequest,
				Message: fmt.Sprintf("invalid page value %q: must be a positive integer", raw),
			}
		}
	}

	if raw := r.URL.Query().Get("per_page"); raw != "" {
		perPage, err = strconv.Atoi(raw)
		if err != nil || perPage < 1 {
			return 0, 0, &APIError{
				Code:    ErrCodeBadRequest,
				Message: fmt.Sprintf("invalid per_page value %q: must be a positive integer", raw),
			}
		}
		if perPage > maxPerPage {
			perPage = maxPerPage
		}
	}

	return page, perPage, nil
}

// PaginateSlice returns the sub-slice of items for the requested page and builds
// the corresponding PaginationMeta.
//
// If the slice is shorter than a full page, the remaining items are returned.
// If page is beyond the last page, an empty slice is returned.
func PaginateSlice[T any](items []T, page, perPage int) ([]T, *PaginationMeta) {
	total := len(items)

	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	meta := &PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}

	start := (page - 1) * perPage
	if start >= total {
		return []T{}, meta
	}

	end := start + perPage
	if end > total {
		end = total
	}

	return items[start:end], meta
}

package knowledgeapi

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

// normalizePage applies knowledgeapi's pagination defaults and caps,
// mirroring packages/gateway.ParsePagination's behaviour exactly (same
// defaults, same per_page ceiling) so a caller adapting a gateway HTTP
// handler onto this package sees consistent semantics. Returns
// ErrInvalidPagination if either field is explicitly negative.
func normalizePage(req PageRequest) (page, perPage int, err error) {
	page = req.Page
	if page == 0 {
		page = defaultPage
	}
	if page < 1 {
		return 0, 0, ErrInvalidPagination
	}

	perPage = req.PerPage
	if perPage == 0 {
		perPage = defaultPerPage
	}
	if perPage < 1 {
		return 0, 0, ErrInvalidPagination
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	return page, perPage, nil
}

// paginate returns the sub-slice of items for the requested page and
// builds the corresponding PageMeta, mirroring
// packages/gateway.PaginateSlice's semantics.
func paginate[T any](items []T, page, perPage int) ([]T, PageMeta) {
	total := len(items)

	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	meta := PageMeta{
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

package handler

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

type Pagination struct {
	Page     int
	PageSize int
	Sort     string
	Order    string
}

type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

func ParsePagination(r *http.Request) Pagination {
	query := r.URL.Query()

	page := parsePositiveInt(query.Get("page"), defaultPage)
	pageSize := parsePositiveInt(query.Get("page_size"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	order := strings.ToLower(strings.TrimSpace(query.Get("order")))
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	return Pagination{
		Page:     page,
		PageSize: pageSize,
		Sort:     strings.TrimSpace(query.Get("sort")),
		Order:    order,
	}
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

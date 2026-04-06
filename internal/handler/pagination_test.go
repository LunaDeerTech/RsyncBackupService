package handler

import (
	"net/http/httptest"
	"testing"
)

func TestParsePaginationDefaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/instances", nil)

	pagination := ParsePagination(req)

	if pagination.Page != 1 {
		t.Fatalf("Page = %d, want %d", pagination.Page, 1)
	}
	if pagination.PageSize != 20 {
		t.Fatalf("PageSize = %d, want %d", pagination.PageSize, 20)
	}
	if pagination.Sort != "" {
		t.Fatalf("Sort = %q, want empty", pagination.Sort)
	}
	if pagination.Order != "desc" {
		t.Fatalf("Order = %q, want %q", pagination.Order, "desc")
	}
}

func TestParsePaginationClampsAndSanitizes(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/instances?page=3&page_size=500&sort=created_at&order=ASC", nil)

	pagination := ParsePagination(req)

	if pagination.Page != 3 {
		t.Fatalf("Page = %d, want %d", pagination.Page, 3)
	}
	if pagination.PageSize != 100 {
		t.Fatalf("PageSize = %d, want %d", pagination.PageSize, 100)
	}
	if pagination.Sort != "created_at" {
		t.Fatalf("Sort = %q, want %q", pagination.Sort, "created_at")
	}
	if pagination.Order != "asc" {
		t.Fatalf("Order = %q, want %q", pagination.Order, "asc")
	}
}

func TestParsePaginationFallsBackOnInvalidValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/instances?page=-1&page_size=abc&order=sideways", nil)

	pagination := ParsePagination(req)

	if pagination.Page != 1 {
		t.Fatalf("Page = %d, want %d", pagination.Page, 1)
	}
	if pagination.PageSize != 20 {
		t.Fatalf("PageSize = %d, want %d", pagination.PageSize, 20)
	}
	if pagination.Order != "desc" {
		t.Fatalf("Order = %q, want %q", pagination.Order, "desc")
	}
}

package api

import (
	"math"
	"net/url"
	"strconv"
)

type meta struct {
	Count int `json:"count"`
}

type navigationLinks struct {
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
	Next  string `json:"next,omitempty"`
	Prev  string `json:"prev,omitempty"`
}

type paginatedResponse struct {
	Meta  meta            `json:"meta"`
	Links navigationLinks `json:"links"`
	Data  interface{}     `json:"data"`
}

func buildPaginatedResponse(u *url.URL, offset int, limit int, total int, data interface{}) *paginatedResponse {
	m := meta{Count: total}
	l := buildNavigationLinks(u, offset, limit, total)
	return &paginatedResponse{Meta: m, Links: *l, Data: data}

}

func buildNavigationLink(originalUrl *url.URL, offset int, limit int) string {
	// make a copy of the original url
	copiedUrl, _ := url.Parse(originalUrl.String())
	values := copiedUrl.Query()
	values.Set("offset", strconv.Itoa(offset))
	values.Set("limit", strconv.Itoa(limit))
	copiedUrl.RawQuery = values.Encode()
	return copiedUrl.String()
}

func buildNavigationLinks(u *url.URL, offset, limit, total int) *navigationLinks {
	firstOffset := 0
	lastOffset := calculateOffsetOfLastPage(total, limit)
	nextOffset := offset + limit
	previousOffset := offset - limit
	if previousOffset < 0 {
		previousOffset = 0
	}

	if total == 0 {
		return &navigationLinks{}
	}

	l := navigationLinks{
		First: buildNavigationLink(u, firstOffset, limit),
		Last:  buildNavigationLink(u, lastOffset, limit),
	}

	if nextOffset < total {
		l.Next = buildNavigationLink(u, nextOffset, limit)
	}

	if offset > 0 {
		l.Prev = buildNavigationLink(u, previousOffset, limit)
	}

	return &l
}

func calculateOffsetOfLastPage(total int, limit int) int {
	lastPage := int(math.Floor(float64(math.Max(float64(total-1), 0)) / float64(limit)))
	return lastPage * limit
}

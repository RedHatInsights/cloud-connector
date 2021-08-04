package api

import (
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
	first_offset := 0
	last_offset := total - 1
	next_offset := offset + limit
	prev_offset := offset - limit
	if prev_offset < 0 {
		prev_offset = 0
	}

	if total == 0 {
		return &navigationLinks{}
	}

	l := navigationLinks{
		First: buildNavigationLink(u, first_offset, limit),
		Last:  buildNavigationLink(u, last_offset, limit),
	}

	if next_offset < total {
		l.Next = buildNavigationLink(u, next_offset, limit)
	}

	if offset > 0 {
		l.Prev = buildNavigationLink(u, prev_offset, limit)
	}

	return &l
}

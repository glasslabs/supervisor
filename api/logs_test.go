package api_test

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		lines     []string
		followCh  chan string
		wantLines []string
	}{
		{
			name:      "buffered lines",
			url:       "/glass/logs",
			lines:     []string{"line one", "line two"},
			wantLines: []string{"line one", "line two"},
		},
		{
			name:      "no lines",
			url:       "/glass/logs",
			wantLines: nil,
		},
		{
			name:      "follow streams additional lines",
			url:       "/glass/logs?follow=true",
			lines:     []string{"buffered"},
			followCh:  make(chan string, 2),
			wantLines: []string{"buffered", "streamed one", "streamed two"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sup := new(mockSupervisor)
			sup.On("Lines").Return(test.lines)

			if test.followCh != nil {
				test.followCh <- "streamed one"
				test.followCh <- "streamed two"
				close(test.followCh)
				sup.On("Follow").Return((<-chan string)(test.followCh))
			}

			srv := newServer(sup, nil, "", t.TempDir())

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.url, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

			var got []string
			scanner := bufio.NewScanner(w.Body)
			for scanner.Scan() {
				got = append(got, scanner.Text())
			}
			require.NoError(t, scanner.Err())

			assert.Equal(t, test.wantLines, got)

			sup.AssertExpectations(t)
		})
	}
}


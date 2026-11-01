package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/zeromicro/go-zero/core/jsonx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestMaxBytesHandler(t *testing.T) {
	maxb := MaxBytesHandler(10)
	handler := maxb(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost",
		bytes.NewBufferString("123456789012345"))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)

	req = httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewBufferString("12345"))
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestMaxBytesHandlerNoLimit(t *testing.T) {
	maxb := MaxBytesHandler(-1)
	handler := maxb(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost",
		bytes.NewBufferString("123456789012345"))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestMaxBytesHandler_ReadBody(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		contentLength int64
		expectCode    int
	}{
		{
			name:          "known length within limit",
			body:          `{"a":1}`,
			contentLength: 7,
			expectCode:    http.StatusOK,
		},
		{
			name:          "known length exceeds limit",
			body:          `{"a":1234567890}`,
			contentLength: 16,
			expectCode:    http.StatusRequestEntityTooLarge,
		},
		{
			name:          "unknown length within limit",
			body:          `{"a":1}`,
			contentLength: -1,
			expectCode:    http.StatusOK,
		},
		{
			name:          "unknown length at limit",
			body:          `{"a":12}`,
			contentLength: -1,
			expectCode:    http.StatusOK,
		},
		{
			name:          "unknown length exceeds limit",
			body:          `{"a":1234567890}`,
			contentLength: -1,
			expectCode:    http.StatusRequestEntityTooLarge,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := MaxBytesHandler(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var v map[string]int
				if err := jsonx.UnmarshalFromReader(r.Body, &v); err != nil {
					httpx.ErrorCtx(r.Context(), w, err)
					return
				}

				assert.NotZero(t, v["a"])
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(test.body))
			req.ContentLength = test.contentLength
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)
			assert.Equal(t, test.expectCode, resp.Code)
		})
	}
}

func TestMaxBytesHandler_ReadError(t *testing.T) {
	handler := MaxBytesHandler(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		httpx.Error(w, err)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", iotest.ErrReader(io.ErrUnexpectedEOF))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestMaxBytesHandler_NoRead(t *testing.T) {
	body := bytes.NewBufferString("123456789012345")
	handler := MaxBytesHandler(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, 15, body.Len())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(body))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusNoContent, resp.Code)
	assert.Equal(t, 15, body.Len())
}

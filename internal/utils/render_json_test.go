package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderJSONWithNilPayload(t *testing.T) {
	w := httptest.NewRecorder()
	RenderJSON(w, 204, nil)

	assert.Equal(t, 204, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Empty(t, w.Body.String())
}

func TestRenderJSONMarshalError(t *testing.T) {
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		RenderJSON(w, 200, make(chan int))
	})
}

func TestRenderJSONWithPayload(t *testing.T) {
	w := httptest.NewRecorder()
	payload := map[string]string{"hello": "world"}
	RenderJSON(w, 200, payload)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.JSONEq(t, `{"hello":"world"}`, w.Body.String())
}

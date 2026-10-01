package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requestWithCookies(rec *httptest.ResponseRecorder) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		r.AddCookie(c)
	}
	return r
}

func TestFlashRoundTrip(t *testing.T) {
	for _, f := range []Flash{
		{Kind: FlashSuccess, Message: "Saved: all good, 100%."},
		{Kind: FlashError, Message: "Response code must be between 100 and 599"},
	} {
		rec := httptest.NewRecorder()
		setFlash(rec, f)

		got := PopFlash(httptest.NewRecorder(), requestWithCookies(rec))
		assert.Equal(t, &f, got)
	}
}

func TestPopFlash_None(t *testing.T) {
	rec := httptest.NewRecorder()
	assert.Nil(t, PopFlash(rec, httptest.NewRequest(http.MethodGet, "/", nil)))
	assert.Empty(t, rec.Result().Cookies(), "nothing to clear")
}

func TestPopFlash_RejectsTamperedCookies(t *testing.T) {
	for _, value := range []string{"not base64!", "bm9jb2xvbg", "aW5mbzpoaQ", "c3VjY2Vzczo"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: flashCookieName, Value: value})
		assert.Nil(t, PopFlash(httptest.NewRecorder(), r), value)
	}
}

// A browser drops a cookie over 4096 bytes, and with it the notice, so a
// long message is cut short instead.
func TestSetFlash_KeepsCookieUnderBrowserLimit(t *testing.T) {
	for _, message := range []string{strings.Repeat("a", 5000), strings.Repeat("é", 5000)} {
		rec := httptest.NewRecorder()
		SetFlashError(rec, message)

		header := rec.Result().Header.Get("Set-Cookie")
		name, value, _ := strings.Cut(strings.SplitN(header, ";", 2)[0], "=")
		assert.Less(t, len(name)+len(value), 4096)

		got := PopFlash(httptest.NewRecorder(), requestWithCookies(rec))
		require.NotNil(t, got)
		assert.Equal(t, FlashError, got.Kind)
		assert.True(t, strings.HasSuffix(got.Message, "..."), got.Message)
		assert.True(t, strings.HasPrefix(message, strings.TrimSuffix(got.Message, "...")))
	}
}

package utils

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// FlashKind is the tone of a Flash notice.
type FlashKind string

const (
	FlashSuccess FlashKind = "success"
	FlashError   FlashKind = "error"
)

// A Flash is a one-time notice shown on the next page the user loads,
// typically after a redirect.
type Flash struct {
	Kind    FlashKind
	Message string
}

const flashCookieName = "_webhook_tester_flash"

// maxFlashMessage bounds a Flash's message, in bytes, so its cookie stays
// well under the 4096 bytes browsers keep; a longer message is cut short.
const maxFlashMessage = 1024

// SetFlashSuccess queues a success notice for the next page render.
func SetFlashSuccess(w http.ResponseWriter, message string) {
	setFlash(w, Flash{Kind: FlashSuccess, Message: message})
}

// SetFlashError queues an error notice for the next page render.
func SetFlashError(w http.ResponseWriter, message string) {
	setFlash(w, Flash{Kind: FlashError, Message: message})
}

func setFlash(w http.ResponseWriter, f Flash) {
	f.Message = Abbreviate(f.Message, maxFlashMessage)
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(string(f.Kind) + ":" + f.Message)),
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// PopFlash returns the queued notice, if any, and clears it. It must run
// before the response body is written.
func PopFlash(w http.ResponseWriter, r *http.Request) *Flash {
	c, err := r.Cookie(flashCookieName)
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: flashCookieName, Path: "/", MaxAge: -1})

	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	k, message, ok := strings.Cut(string(raw), ":")
	kind := FlashKind(k)
	if !ok || message == "" || (kind != FlashSuccess && kind != FlashError) {
		return nil
	}
	return &Flash{Kind: kind, Message: message}
}

package utils

import (
	"github.com/unrolled/render"
	"log"
	"net/http"
)

func RenderJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")

	if payload == nil {
		w.WriteHeader(status)
		return
	}

	err := render.New().JSON(w, status, payload)
	if err != nil {
		log.Print("error rendering json: ", err)
	}
}

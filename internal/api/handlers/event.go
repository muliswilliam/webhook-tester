package handlers

import "net/http"

func ReceiveEvent(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("receive event"))
}

func ListEvents(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("list events"))
}

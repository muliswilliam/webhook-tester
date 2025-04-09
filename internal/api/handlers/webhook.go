package handlers

import "net/http"

func CreateWebhook(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Create webhook"))
}

func ListWebhooks(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("List webhooks"))
}

func GetWebhook(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Get webook by ID"))
}

func UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Update webook by ID"))
}

func DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Delete webook by ID"))
}

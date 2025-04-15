package web

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"

	"github.com/wader/gormstore/v2"
)

var SessionStore *gormstore.Store
var SessionName = "_webhook_tester_session_id"

func CreateSessionStore() {
	SessionStore = gormstore.New(db.DB, []byte(os.Getenv("AUTH_SECRET")))
	// db cleanup every 2 days
	// close quit channel to stop cleanup
	quit := make(chan struct{})
	go SessionStore.PeriodicCleanup(48*time.Hour, quit)
}

func Authorize(r *http.Request) error {
	authError := errors.New("unauthorized")
	sess, err := SessionStore.Get(r, SessionName)

	if err != nil {
		return authError
	}

	userID, ok := sess.Values["user_id"].(string)

	if !ok || userID == "" {
		return authError
	}

	return nil
}

func GetLoggedInUser(r *http.Request) models.User {
	var user models.User
	sess, err := SessionStore.Get(r, SessionName)
	if err != nil {
		log.Printf("error getting session %s", err)
	}

	log.Printf("%+v\n", sess.Values)

	userID, ok := sess.Values["user_id"]
	log.Printf("userid %s", userID)
	if !ok || userID == "" {
		log.Printf("no logged in user")
		return user
	}

	err = db.DB.First(&user, userID).Error
	if err != nil {
		log.Printf("error getting user %s", err)
	}

	return user
}

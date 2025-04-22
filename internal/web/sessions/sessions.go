package sessions

import (
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/models"

	"github.com/wader/gormstore/v2"
)

var Name = "_webhook_tester_session_id"

func CreateSessionStore(db *gorm.DB) *gormstore.Store {
	store := gormstore.New(db, []byte(os.Getenv("AUTH_SECRET")))
	// db cleanup every 2 days
	// close quit channel to stop cleanup
	quit := make(chan struct{})
	go store.PeriodicCleanup(48*time.Hour, quit)
	return store
}

func Authorize(r *http.Request, store *gormstore.Store) (uint, error) {
	authError := errors.New("unauthorized")
	sess, err := store.Get(r, Name)

	if err != nil {
		return 0, authError
	}
	userIDRaw := sess.Values["user_id"]
	userID, ok := userIDRaw.(uint)

	if !ok {
		return 0, authError
	}

	return userID, nil
}

func GetLoggedInUser(r *http.Request, store *gormstore.Store, db *gorm.DB) models.User {
	var user models.User
	sess, err := store.Get(r, Name)
	if err != nil {
		log.Printf("error getting session %s", err)
	}

	userID, ok := sess.Values["user_id"]
	if !ok || userID == "" {
		log.Printf("no logged in user")
		return user
	}

	err = db.First(&user, userID).Error
	if err != nil {
		log.Printf("error getting user %s", err)
	}

	return user
}

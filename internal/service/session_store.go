package service

import (
	"errors"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/wader/gormstore/v2"
)

type SessionStore interface {
	Get(r *http.Request, sessionName string) (*sessions.Session, error)
	GetValue(r *http.Request, sessionName, key string) (interface{}, error)
	New(r *http.Request, w http.ResponseWriter, sessionName string, key string, value interface{}, options sessions.Options) (*sessions.Session, error)
	Exists(r *http.Request, sessionName string) (bool, error)
	Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error
	Delete(r *http.Request, w http.ResponseWriter, sessionName string) error
}

type sessionStore struct {
	store *gormstore.Store
}

func NewSessionStore(store *gormstore.Store) SessionStore {
	if store == nil {
		log.Fatal("session store is nil")
	}
	return &sessionStore{store: store}
}

func (s *sessionStore) Get(r *http.Request, sessionName string) (*sessions.Session, error) {
	return s.store.Get(r, sessionName)
}

func (s *sessionStore) GetValue(r *http.Request, sessionName, key string) (interface{}, error) {
	sess, err := s.store.Get(r, sessionName)
	if err != nil {
		return nil, err
	}
	raw, ok := sess.Values[key]
	if !ok {
		return nil, errors.New("key not found")
	}
	return raw, nil
}

func (s *sessionStore) New(r *http.Request, w http.ResponseWriter, sessionName string, key string, value interface{}, options sessions.Options) (*sessions.Session, error) {
	sess, err := s.store.Get(r, sessionName)
	if err != nil {
		return nil, err
	}

	sess.Values[key] = value
	sess.Options = &options

	err = s.Save(r, w, sess)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *sessionStore) Exists(r *http.Request, name string) (bool, error) {
	_, err := s.store.Get(r, name)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *sessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	return s.store.Save(r, w, session)
}

func (s *sessionStore) Delete(r *http.Request, w http.ResponseWriter, sessionName string) error {
	sess, err := s.store.Get(r, sessionName)
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1
	return s.Save(r, w, sess)
}

package service

import (
	"errors"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/wader/gormstore/v2"
)

type SessionStore interface {
	Get(r *http.Request, sessionName string) (*sessions.Session, error)
	GetValue(r *http.Request, sessionName, key string) (interface{}, error)
	New(r *http.Request, w http.ResponseWriter, sessionName string, key string, value interface{}, options sessions.Options) (*sessions.Session, error)
	Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error
	Delete(r *http.Request, w http.ResponseWriter, sessionName string) error
}

type Store interface {
	Get(r *http.Request, sessionName string) (*sessions.Session, error)
	Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error
}

type GormStore struct {
	store *gormstore.Store
}

func NewGormStore(store *gormstore.Store) Store {
	return &GormStore{store: store}
}

func (s *GormStore) Get(r *http.Request, sessionName string) (*sessions.Session, error) {
	return s.store.Get(r, sessionName)
}

func (s *GormStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	return s.store.Save(r, w, session)
}

type sessionStore struct {
	store Store
}

func NewSessionStore(store Store) SessionStore {
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

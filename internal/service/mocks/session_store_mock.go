package serviceMocks

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"
)

type SessionStoreMock struct {
	mock.Mock
}

func (m *SessionStoreMock) Get(r *http.Request, sessionName string) (*sessions.Session, error) {
	args := m.Called(r, sessionName)
	return args.Get(0).(*sessions.Session), args.Error(1)
}

func (m *SessionStoreMock) GetValue(r *http.Request, sessionName, key string) (interface{}, error) {
	args := m.Called(r, sessionName, key)
	return args.Get(0), args.Error(1)
}

func (m *SessionStoreMock) New(r *http.Request, w http.ResponseWriter, sessionName string, key string, value interface{}, options sessions.Options) (*sessions.Session, error) {
	args := m.Called(r, w, sessionName, key, value, options)
	return args.Get(0).(*sessions.Session), args.Error(1)
}

func (m *SessionStoreMock) Exists(r *http.Request, name string) (bool, error) {
	args := m.Called(r, name)
	return args.Get(0).(bool), args.Error(1)
}

func (m *SessionStoreMock) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	args := m.Called(r, w, session)
	return args.Error(0)
}

func (m *SessionStoreMock) Delete(r *http.Request, w http.ResponseWriter, sessionName string) error {
	args := m.Called(r, w, sessionName)
	return args.Error(0)
}

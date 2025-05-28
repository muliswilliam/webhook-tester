package service_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"webhook-tester/internal/service"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type GormStoreMock struct {
	mock.Mock
}

func (m *GormStoreMock) Get(r *http.Request, sessionName string) (*sessions.Session, error) {
	args := m.Called(r, sessionName)
	return args.Get(0).(*sessions.Session), args.Error(1)
}

func (m *GormStoreMock) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	args := m.Called(r, w, session)
	return args.Error(0)
}

type SessionStoreTestSuite struct {
	suite.Suite
	mockStore *GormStoreMock
	svc       service.SessionStore
}

func (suite *SessionStoreTestSuite) SetupTest() {
	suite.mockStore = new(GormStoreMock)
	suite.svc = service.NewSessionStore(suite.mockStore)
}

func (suite *SessionStoreTestSuite) TearDownSuite() {
	suite.mockStore = nil
	suite.svc = nil
}

func TestSessionStoreTestSuite(t *testing.T) {
	suite.Run(t, new(SessionStoreTestSuite))
}

func (suite *SessionStoreTestSuite) TestSessionStore_New() {
	suite.mockStore.On("Get", mock.Anything, "sessionName").Return(&sessions.Session{
		Options: &sessions.Options{},
		Values:  map[interface{}]interface{}{},
	}, nil)
	suite.mockStore.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	sessionName := "sessionName"
	session, err := suite.svc.New(r, w, sessionName, "key", "value", sessions.Options{})
	suite.NoError(err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.NotNil(session)
	suite.Equal("value", session.Values["key"])
}

func (suite *SessionStoreTestSuite) TestSessionStore_New_SessionNotFound() {
	suite.mockStore.On("Get", mock.Anything, "sessionName").Return(&sessions.Session{
		Options: &sessions.Options{},
		Values:  map[interface{}]interface{}{},
	}, errors.New("not found"))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	sessionName := "sessionName"
	_, err := suite.svc.New(r, w, sessionName, "key", "value", sessions.Options{})
	suite.Error(err)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *SessionStoreTestSuite) TestSessionStore_GetValue() {
	sessionName := "sessionName"
	suite.mockStore.On("Get", mock.Anything, sessionName).Return(&sessions.Session{
		Values: map[interface{}]interface{}{
			"key": "value",
		},
	}, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	value, err := suite.svc.GetValue(r, sessionName, "key")
	suite.NoError(err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.Equal("value", value)
}

func (suite *SessionStoreTestSuite) TestSessionStore_GetValue_KeyNotFound() {
	sessionName := "sessionName"
	suite.mockStore.On("Get", mock.Anything, sessionName).Return(&sessions.Session{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	value, err := suite.svc.GetValue(r, sessionName, "key")
	suite.Error(err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.Nil(value)
}

func (suite *SessionStoreTestSuite) TestSessionStore_Save() {
	suite.mockStore.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	session := &sessions.Session{
		Values: map[interface{}]interface{}{
			"key": "value",
		},
	}
	err := suite.svc.Save(r, w, session)
	suite.NoError(err)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *SessionStoreTestSuite) TestSessionStore_Save_Error() {
	suite.mockStore.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("save error"))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	session := &sessions.Session{
		Values: map[interface{}]interface{}{
			"key": "value",
		},
	}
	err := suite.svc.Save(r, w, session)
	suite.Error(err)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *SessionStoreTestSuite) TestSessionStore_Delete() {
	sessionName := "sessionName"
	suite.mockStore.On("Get", mock.Anything, sessionName).Return(&sessions.Session{
		Options: &sessions.Options{},
	}, nil)
	suite.mockStore.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	err := suite.svc.Delete(r, w, sessionName)
	suite.NoError(err)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *SessionStoreTestSuite) TestSessionStore_Delete_Error() {
	suite.mockStore.On("Get", mock.Anything, "sessionName").Return(&sessions.Session{}, errors.New("not found"))
	suite.mockStore.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("save error"))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	sessionName := "sessionName"
	err := suite.svc.Delete(r, w, sessionName)
	suite.Error(err)
}

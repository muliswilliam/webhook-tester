package service_test

import (
	"errors"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"testing"
	"webhook-tester/internal/models"
	repositoryMocks "webhook-tester/internal/repository/mocks"
	"webhook-tester/internal/service"
)

type WebhookServiceTestSuite struct {
	suite.Suite
	mockRepo repositoryMocks.WebhookRepositoryMock
	svc      service.WebhookService
}

func (suite *WebhookServiceTestSuite) SetupTest() {
	suite.mockRepo = repositoryMocks.WebhookRepositoryMock{}
	suite.svc = service.NewWebhookService(&suite.mockRepo)
}

func TestWebhookServiceTestSuite(t *testing.T) {
	suite.Run(t, new(WebhookServiceTestSuite))
}

func (suite *WebhookServiceTestSuite) TestWebookService_CreateWebhook() {
	suite.mockRepo.On("Insert", mock.Anything).Return(nil)
	err := suite.svc.CreateWebhook(&models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1})
	suite.NoError(err)
}

func (suite *WebhookServiceTestSuite) TestWebookService_CreateWebhook_Error() {
	suite.mockRepo.On("Insert", mock.Anything).Return(errors.New("insert error"))
	err := suite.svc.CreateWebhook(&models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1})
	suite.Error(err)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_Get() {
	suite.mockRepo.On("Get", "id").Return(&models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1}, nil)
	wh, err := suite.svc.GetWebhook("id")
	suite.NoError(err)
	suite.Equal(&models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1}, wh)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_Get_Error() {
	suite.mockRepo.On("Get", "missing").Return(&models.Webhook{}, errors.New("not found"))
	_, err := suite.svc.GetWebhook("missing")
	suite.Error(err)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_ListByUser() {
	suite.mockRepo.On("GetAllByUser", uint(42)).Return([]models.Webhook{
		{ID: "w1"},
		{ID: "w2"},
	}, nil)
	list, err := suite.svc.ListWebhooks(42)
	suite.NoError(err)
	suite.Equal([]models.Webhook{
		{ID: "w1"},
		{ID: "w2"},
	}, list)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_ListByUser_Error() {
	suite.mockRepo.On("GetAllByUser", uint(100)).Return([]models.Webhook{}, errors.New("db error"))
	_, err := suite.svc.ListWebhooks(100)
	suite.Error(err)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_Update() {
	suite.mockRepo.On("Update", mock.Anything).Return(nil)
	err := suite.svc.UpdateWebhook(&models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1})
	suite.NoError(err)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_Update_Error() {
	suite.mockRepo.On("Update", mock.Anything).Return(errors.New("update error"))
	err := suite.svc.UpdateWebhook(&models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1})
	suite.Error(err)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_Delete() {
	suite.mockRepo.On("Delete", "id", uint(1)).Return(nil)
	err := suite.svc.DeleteWebhook("id", 1)
	suite.NoError(err)
}

func (suite *WebhookServiceTestSuite) TestWebhookService_Delete_Error() {
	suite.mockRepo.On("Delete", "id", uint(1)).Return(errors.New("delete error"))
	err := suite.svc.DeleteWebhook("id", 1)
	suite.Error(err)
}

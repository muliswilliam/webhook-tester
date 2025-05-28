package service_test

import (
	"testing"
	"webhook-tester/internal/models"
	repositoryMocks "webhook-tester/internal/repository/mocks"
	"webhook-tester/internal/service"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type WebhookRequestServiceTestSuite struct {
	suite.Suite
	mockRepo repositoryMocks.WebhookRequestRepositoryMock
	svc      service.WebhookRequestService
}

func (suite *WebhookRequestServiceTestSuite) SetupTest() {
	suite.mockRepo = repositoryMocks.WebhookRequestRepositoryMock{}
	suite.svc = service.NewWebhookRequestService(&suite.mockRepo)
}

func TestWebhookRequestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(WebhookRequestServiceTestSuite))
}

func (suite *WebhookRequestServiceTestSuite) TestWebhookRequestService_Record() {
	suite.mockRepo.On("Insert", mock.Anything).Return(nil)
	err := suite.svc.Record(&models.WebhookRequest{})
	suite.NoError(err)
}

func (suite *WebhookRequestServiceTestSuite) TestWebhookRequestService_Get() {
	suite.mockRepo.On("GetByID", "id").Return(&models.WebhookRequest{}, nil)
	req, err := suite.svc.Get("id")
	suite.NoError(err)
	suite.NotNil(req)
}

func (suite *WebhookRequestServiceTestSuite) TestWebhookRequestService_List() {
	suite.mockRepo.On("ListByWebhook", "webhookID").Return([]models.WebhookRequest{}, nil)
	reqs, err := suite.svc.List("webhookID")
	suite.NoError(err)
	suite.NotNil(reqs)
}

func (suite *WebhookRequestServiceTestSuite) TestWebhookRequestService_Delete() {
	suite.mockRepo.On("DeleteByID", "id").Return(nil)
	err := suite.svc.Delete("id")
	suite.NoError(err)
}

func (suite *WebhookRequestServiceTestSuite) TestWebhookRequestService_DeleteAll() {
	suite.mockRepo.On("DeleteByWebhook", "webhookID").Return(nil)
	err := suite.svc.DeleteAll("webhookID")
	suite.NoError(err)
}
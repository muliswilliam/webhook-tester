package service_test

// func TestWebookService_CreateWebhook(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)
// 	wh := &models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1}
// 	mockRepo.EXPECT().Insert(wh).Return(nil)
// 	err := svc.CreateWebhook(wh)
// 	assert.NoError(t, err)
// }

// func TestWebookService_Create_Error(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)
// 	wh := &models.Webhook{ID: "id", Title: "title", ResponseCode: 200, UserID: 1}
// 	mockRepo.EXPECT().Insert(wh).Return(gorm.ErrRecordNotFound)
// 	err := svc.CreateWebhook(wh)
// 	assert.Equal(t, gorm.ErrRecordNotFound, err)
// }

// func TestWebhookService_Get(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	expected := &models.Webhook{ID: "w3", Title: "Fetch"}
// 	mockRepo.EXPECT().Get("w3").Return(expected, nil)

// 	result, err := svc.GetWebhook("w3")
// 	assert.NoError(t, err)
// 	assert.Equal(t, expected, result)
// }

// func TestWebhookService_Get_Error(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	repoErr := errors.New("not found")
// 	mockRepo.EXPECT().Get("missing").Return(nil, repoErr)

// 	result, err := svc.GetWebhook("missing")
// 	assert.Nil(t, result)
// 	assert.EqualError(t, err, "not found")
// }

// func TestWebhookService_ListByUser(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	exList := []models.Webhook{
// 		{ID: "w1"},
// 		{ID: "w2"},
// 	}
// 	mockRepo.EXPECT().GetAllByUser(uint(42)).Return(exList, nil)

// 	list, err := svc.ListWebhooks(42)
// 	assert.NoError(t, err)
// 	assert.Equal(t, exList, list)
// }

// func TestWebhookService_ListByUser_Error(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	repoErr := errors.New("db error")
// 	mockRepo.EXPECT().GetAllByUser(uint(100)).Return(nil, repoErr)

// 	list, err := svc.ListWebhooks(100)
// 	assert.Nil(t, list)
// 	assert.EqualError(t, err, "db error")
// }

// func TestWebhookService_Update(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	wh := &models.Webhook{ID: "w4"}
// 	mockRepo.EXPECT().Update(wh).Return(nil)

// 	err := svc.UpdateWebhook(wh)
// 	assert.NoError(t, err)
// }

// func TestWebhookService_Update_Error(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	wh := &models.Webhook{ID: "w5"}
// 	repoErr := errors.New("update failed")
// 	mockRepo.EXPECT().Update(wh).Return(repoErr)

// 	err := svc.UpdateWebhook(wh)
// 	assert.EqualError(t, err, "update failed")
// }

// func TestWebhookService_Delete(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	mockRepo.EXPECT().Delete("w6", uint(200)).Return(nil)

// 	err := svc.DeleteWebhook("w6", 200)
// 	assert.NoError(t, err)
// }

// func TestWebhookService_Delete_Error(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockRepo := mocks.NewMockWebhookRepository(ctrl)
// 	svc := service.NewWebhookService(mockRepo)

// 	repoErr := errors.New("delete failed")
// 	mockRepo.EXPECT().Delete("w7", uint(200)).Return(repoErr)

// 	err := svc.DeleteWebhook("w7", 200)
// 	assert.EqualError(t, err, "delete failed")
// }

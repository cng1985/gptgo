package account

func AccountEntityToResponse(entity *Account, provider string, ready bool) *AccountResponse {
	return &AccountResponse{
		ID:            entity.ID,
		Name:          entity.Name,
		Username:      entity.Username,
		Provider:      provider,
		ProviderReady: ready,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
}

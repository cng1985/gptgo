package chatbi

func DatasourceEntityToSimple(entity *Datasource) *DatasourceSimple {
	return &DatasourceSimple{
		ID:          entity.ID,
		Name:        entity.Name,
		Driver:      entity.Driver,
		Host:        entity.Host,
		Port:        entity.Port,
		Database:    entity.DatabaseName,
		Username:    entity.Username,
		SSLMode:     entity.SSLMode,
		PasswordSet: entity.PasswordCipher != "",
		UpdatedAt:   entity.UpdatedAt,
	}
}

func DatasourceEntityToResponse(entity *Datasource) *DatasourceResponse {
	return &DatasourceResponse{
		ID:          entity.ID,
		Name:        entity.Name,
		Driver:      entity.Driver,
		Host:        entity.Host,
		Port:        entity.Port,
		Database:    entity.DatabaseName,
		Username:    entity.Username,
		SSLMode:     entity.SSLMode,
		PasswordSet: entity.PasswordCipher != "",
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func BillingEntityToSimple(entity *BillingEntry) *BillingSimple {
	return &BillingSimple{
		ID:           entity.ID,
		Kind:         entity.Kind,
		Amount:       entity.Amount,
		BalanceAfter: entity.BalanceAfter,
		RefType:      entity.RefType,
		RefID:        entity.RefID,
		Note:         entity.Note,
		CreatedAt:    entity.CreatedAt,
	}
}

func UsageEntityToSimple(entity *UsageRecord) *UsageSimple {
	return &UsageSimple{
		ID:               entity.ID,
		DatasourceID:     entity.DatasourceID,
		DatasourceName:   entity.DatasourceName,
		Question:         entity.Question,
		SQL:              entity.SQLText,
		Status:           entity.Status,
		RowCount:         entity.RowCount,
		DurationMs:       entity.DurationMs,
		PromptTokens:     entity.PromptTokens,
		CompletionTokens: entity.CompletionTokens,
		Cost:             entity.Cost,
		ErrorMessage:     entity.ErrorMessage,
		CreatedAt:        entity.CreatedAt,
	}
}

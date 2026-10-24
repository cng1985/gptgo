package agent

import "encoding/json"

func decodeSuggestions(raw string) []string {
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil || items == nil {
		return []string{}
	}
	return items
}

func decodeIDs(raw string) []int64 {
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil || ids == nil {
		return []int64{}
	}
	return ids
}

func encodeIDs(ids []int64) string {
	if ids == nil {
		ids = []int64{}
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func encodeSuggestions(items []string) string {
	if items == nil {
		items = []string{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func toSimple(entity *Agent) AgentSimple {
	return AgentSimple{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		Model:       entity.Model,
		Opening:     entity.Opening,
		Suggestions: decodeSuggestions(entity.Suggestions),
		UpdatedAt:   entity.UpdatedAt,
	}
}

func toResponse(entity *Agent) AgentResponse {
	return AgentResponse{
		ID:           entity.ID,
		Name:         entity.Name,
		Description:  entity.Description,
		Instructions: entity.Instructions,
		Model:        entity.Model,
		Opening:      entity.Opening,
		Suggestions:  decodeSuggestions(entity.Suggestions),
		Tools:        decodeSuggestions(entity.Tools),
		McpIDs:       IDList(decodeIDs(entity.McpIDs)),
		CreatedAt:    entity.CreatedAt,
		UpdatedAt:    entity.UpdatedAt,
	}
}

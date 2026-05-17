package tools

import (
	"strings"
)

type CustomFieldDefinitionView struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	Status     string `json:"status,omitempty"`
	EntityType string `json:"entity_type,omitempty"`
	Source     string `json:"source,omitempty"`
}

type CustomFieldValueView struct {
	ID            string `json:"id,omitempty"`
	CustomFieldID string `json:"custom_field_id,omitempty"`
	Name          string `json:"name,omitempty"`
	Type          string `json:"type,omitempty"`
	Status        string `json:"status,omitempty"`
	EntityType    string `json:"entity_type,omitempty"`
	Value         any    `json:"value,omitempty"`
	Source        string `json:"source"`
}

func customFieldValuesFromRaw(raw any) []CustomFieldValueView {
	rows := mapSlice(raw)
	if len(rows) == 0 {
		if row, ok := raw.(map[string]any); ok {
			rows = []map[string]any{row}
		}
	}
	out := make([]CustomFieldValueView, 0, len(rows))
	for _, row := range rows {
		if view, ok := customFieldValueFromRaw(row); ok {
			out = append(out, view)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func customFieldValueFromRaw(raw map[string]any) (CustomFieldValueView, bool) {
	if raw == nil {
		return CustomFieldValueView{}, false
	}
	def := customFieldDefinitionFromRaw(raw)
	id := firstReportString(raw, "id", "_id", "customFieldValueId", "custom_field_value_id")
	cfID := firstReportString(raw, "customFieldId", "custom_field_id", "fieldId", "field_id")
	name := firstReportString(raw, "customFieldName", "custom_field_name", "name", "label")
	typ := strings.ToUpper(firstReportString(raw, "customFieldType", "custom_field_type", "type", "fieldType", "field_type"))
	status := strings.ToUpper(firstReportString(raw, "status"))
	entityType := strings.ToUpper(firstReportString(raw, "entityType", "entity_type", "sourceType", "source_type"))
	value := firstPresent(raw, "value", "customFieldValue", "custom_field_value", "selectedValue", "selected_value")
	if value == nil {
		value = firstPresent(raw, "values", "selectedValues", "selected_values")
	}
	source := "embedded"
	if def.ID != "" || def.Name != "" || def.Type != "" {
		source = "embedded_definition"
	}
	view := CustomFieldValueView{
		ID:            id,
		CustomFieldID: cfID,
		Name:          name,
		Type:          typ,
		Status:        status,
		EntityType:    entityType,
		Value:         value,
		Source:        source,
	}
	if def.ID != "" || def.Name != "" || def.Type != "" || def.Status != "" || def.EntityType != "" {
		if view.CustomFieldID == "" {
			view.CustomFieldID = def.ID
		}
		if view.Name == "" {
			view.Name = def.Name
		}
		if view.Type == "" {
			view.Type = def.Type
		}
		if view.Status == "" {
			view.Status = def.Status
		}
		if view.EntityType == "" {
			view.EntityType = def.EntityType
		}
	}
	if view.ID == "" && view.CustomFieldID == "" && view.Name == "" && view.Type == "" && view.Value == nil {
		return CustomFieldValueView{}, false
	}
	return view, true
}

func customFieldDefinitionFromRaw(raw map[string]any) CustomFieldDefinitionView {
	if raw == nil {
		return CustomFieldDefinitionView{}
	}
	for _, key := range []string{"customField", "definition", "field"} {
		if nested, ok := raw[key].(map[string]any); ok {
			def := customFieldDefinitionFromRaw(nested)
			if def.Source == "" {
				def.Source = "embedded"
			}
			return def
		}
	}
	return CustomFieldDefinitionView{
		ID:         firstReportString(raw, "customFieldId", "custom_field_id", "id", "_id"),
		Name:       firstReportString(raw, "customFieldName", "custom_field_name", "name", "label"),
		Type:       strings.ToUpper(firstReportString(raw, "customFieldType", "custom_field_type", "type", "fieldType", "field_type")),
		Status:     strings.ToUpper(firstReportString(raw, "status")),
		EntityType: strings.ToUpper(firstReportString(raw, "entityType", "entity_type", "sourceType", "source_type")),
		Source:     "embedded",
	}
}

func customFieldsFromFirst(raw map[string]any, keys ...string) []CustomFieldValueView {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if views := customFieldValuesFromRaw(value); len(views) > 0 {
				return views
			}
		}
	}
	return nil
}

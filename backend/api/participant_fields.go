package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ParticipantField struct {
	FieldName string `json:"field_name"`
	Required  bool   `json:"required"`
}

func defaultParticipantForm() []ParticipantField {
	return []ParticipantField{
		{FieldName: "name", Required: true},
		{FieldName: "email", Required: true},
	}
}

func normalizeParticipantFields(fields []ParticipantField) []ParticipantField {
	if len(fields) == 0 {
		return defaultParticipantForm()
	}
	seen := map[string]bool{}
	out := make([]ParticipantField, 0, len(fields))
	for _, f := range fields {
		name := normalizeFieldName(f.FieldName)
		if name == "" {
			continue
		}
		canonical := canonicalFieldName(name)
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		out = append(out, ParticipantField{
			FieldName: name,
			Required:  f.Required,
		})
	}
	if len(out) == 0 {
		return defaultParticipantForm()
	}
	return out
}

func normalizeFieldName(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func canonicalFieldName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func validateParticipantData(fields []ParticipantField, data map[string]interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	for _, field := range fields {
		if !field.Required {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(data[field.FieldName]))
		if value == "" || value == "<nil>" {
			return fmt.Errorf("%s is required", field.FieldName)
		}
	}
	return nil
}

func decodeParticipantFields(raw []byte) []ParticipantField {
	if len(raw) == 0 {
		return defaultParticipantForm()
	}

	var direct []ParticipantField
	if err := json.Unmarshal(raw, &direct); err == nil {
		normalized := normalizeParticipantFields(direct)
		if len(normalized) > 0 {
			return normalized
		}
	}

	// Backward compatibility for legacy shape: {id,label,type,required}
	var legacy []map[string]interface{}
	if err := json.Unmarshal(raw, &legacy); err == nil {
		tmp := make([]ParticipantField, 0, len(legacy))
		for _, row := range legacy {
			name := ""
			if v, ok := row["field_name"].(string); ok {
				name = v
			}
			if name == "" {
				if v, ok := row["id"].(string); ok {
					name = v
				}
			}
			if name == "" {
				if v, ok := row["label"].(string); ok {
					name = v
				}
			}
			required := false
			if v, ok := row["required"].(bool); ok {
				required = v
			}
			tmp = append(tmp, ParticipantField{
				FieldName: name,
				Required:  required,
			})
		}
		return normalizeParticipantFields(tmp)
	}

	return defaultParticipantForm()
}

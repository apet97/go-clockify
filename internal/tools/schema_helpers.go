package tools

const flexibleDatetimeDescription = "Datetime (RFC3339, YYYY-MM-DD, 'today HH:MM', 'yesterday HH:MM', or 'now')"

func timezoneInputProperty() map[string]any {
	return map[string]any{"type": "string", "description": "Optional IANA timezone; defaults to CLOCKIFY_TIMEZONE or the local/server timezone."}
}

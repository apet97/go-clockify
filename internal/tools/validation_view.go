package tools

const (
	validationStatusOK     = "ok"
	validationStatusFailed = "failed"
)

// ValidationView is the validation block attached to a dry-run preview: the
// overall status/ok, any errors and warnings, and a preview-quality label.
type ValidationView struct {
	Status         string              `json:"status"`
	OK             *bool               `json:"ok,omitempty"`
	Errors         []ValidationProblem `json:"errors,omitempty"`
	Warnings       []ValidationProblem `json:"warnings,omitempty"`
	PreviewQuality string              `json:"preview_quality,omitempty"`
}

// ValidationProblem is a single validation error or warning: a code, the
// offending field, a message, and an optional remediation hint with refs.
type ValidationProblem struct {
	Code        string   `json:"code,omitempty"`
	Field       string   `json:"field,omitempty"`
	Message     string   `json:"message"`
	Remediation string   `json:"remediation,omitempty"`
	Refs        []string `json:"refs,omitempty"`
}

func validationOK(quality string) ValidationView {
	ok := true
	if quality == "" {
		quality = "handler_validated"
	}
	return ValidationView{Status: validationStatusOK, OK: &ok, PreviewQuality: quality}
}

func validationFailed(quality string, errors ...ValidationProblem) ValidationView {
	ok := false
	if quality == "" {
		quality = "handler_validated"
	}
	return ValidationView{Status: validationStatusFailed, OK: &ok, Errors: errors, PreviewQuality: quality}
}

func dryrunPreviewPayloadValidated(tool string, payload map[string]any, validation ValidationView) map[string]any {
	preview := dryrunPreviewPayload(tool, payload)
	return attachValidationToPreview(preview, validation)
}

func attachValidationToPreview(preview map[string]any, validation ValidationView) map[string]any {
	if preview == nil {
		preview = map[string]any{}
	}
	preview["validation"] = validation
	switch validation.Status {
	case validationStatusOK:
		preview["validated"] = true
		preview["blocked"] = false
	case validationStatusFailed:
		preview["validated"] = true
		preview["blocked"] = true
	default:
		preview["validated"] = false
	}
	return preview
}

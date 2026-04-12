{{/*
Expand the name of the chart.
*/}}
{{- define "clockify-mcp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this.
*/}}
{{- define "clockify-mcp.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Chart label used by common labels.
*/}}
{{- define "clockify-mcp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every rendered resource.
*/}}
{{- define "clockify-mcp.labels" -}}
helm.sh/chart: {{ include "clockify-mcp.chart" . }}
{{ include "clockify-mcp.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: server
{{- end }}

{{/*
Selector labels — used by Deployment, Service, NetworkPolicy, PDB.
*/}}
{{- define "clockify-mcp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "clockify-mcp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name to use.
*/}}
{{- define "clockify-mcp.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "clockify-mcp.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret name.
*/}}
{{- define "clockify-mcp.secretName" -}}
{{- if .Values.secrets.name }}
{{- .Values.secrets.name }}
{{- else }}
{{- printf "%s-secrets" (include "clockify-mcp.fullname" .) }}
{{- end }}
{{- end }}

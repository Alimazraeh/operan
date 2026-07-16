{{/*
Expand the name of the chart.
*/}}
{{- define "policy-governance.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "policy-governance.fullname" -}}
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
Common labels
*/}}
{{- define "policy-governance.labels" -}}
helm.sh/chart: {{ include "policy-governance.name" . }}-{{ .Chart.Version }}
{{ include "policy-governance.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: policy-governance
{{- end }}

{{/*
Selector labels
*/}}
{{- define "policy-governance.selectorLabels" -}}
app.kubernetes.io/name: {{ include "policy-governance.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "policy-governance.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "policy-governance.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
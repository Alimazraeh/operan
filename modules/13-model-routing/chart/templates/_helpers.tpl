{{- /* _helpers.tpl — shared labels and selectors */ -}}
{{- define "model-routing.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "model-routing.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "model-routing.labels" -}}
app.kubernetes.io/name: {{ include "model-routing.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "model-routing.selectorLabels" -}}
app.kubernetes.io/name: {{ include "model-routing.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
{{- define "module11.labels" -}}
app.kubernetes.io/name: module11-observability
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "module11.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | truncate 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | truncate 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | truncate 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "execution-sandbox.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "execution-sandbox.fullname" -}}
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

{{- define "execution-sandbox.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "execution-sandbox.labels" -}}
helm.sh/chart: {{ include "execution-sandbox.chart" . }}
{{ include "execution-sandbox.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "execution-sandbox.selectorLabels" -}}
app.kubernetes.io/name: {{ include "execution-sandbox.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "execution-sandbox.serviceAccountName" -}}
{{- printf "%s-sa" (include "execution-sandbox.fullname" .) }}
{{- end }}
{{/*
Expand the name of the chart.
*/}}
{{- define "cuearr.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "cuearr.fullname" -}}
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
Chart and version labels.
*/}}
{{- define "cuearr.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cuearr.labels" -}}
helm.sh/chart: {{ include "cuearr.chart" . }}
{{ include "cuearr.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cuearr.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cuearr.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Auth secret name
*/}}
{{- define "cuearr.authSecretName" -}}
{{- if .Values.auth.existingSecret }}
{{- .Values.auth.existingSecret }}
{{- else }}
{{- include "cuearr.fullname" . }}
{{- end }}
{{- end }}

{{/*
Image reference
*/}}
{{- define "cuearr.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Data PVC name
*/}}
{{- define "cuearr.dataPvcName" -}}
{{- printf "%s-data" (include "cuearr.fullname" .) }}
{{- end }}

{{/*
Watch PVC name
*/}}
{{- define "cuearr.watchPvcName" -}}
{{- printf "%s-watch" (include "cuearr.fullname" .) }}
{{- end }}

{{/*
Out PVC name
*/}}
{{- define "cuearr.outPvcName" -}}
{{- printf "%s-out" (include "cuearr.fullname" .) }}
{{- end }}

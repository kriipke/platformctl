{{/*
Expand the name of the chart.
*/}}
{{- define "contextops.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "contextops.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "contextops.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "contextops.labels" -}}
helm.sh/chart: {{ include "contextops.chart" . }}
{{ include "contextops.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: contextops-platform
{{- end }}

{{/*
Selector labels
*/}}
{{- define "contextops.selectorLabels" -}}
app.kubernetes.io/name: {{ include "contextops.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "contextops.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "contextops.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Database URL
*/}}
{{- define "contextops.databaseUrl" -}}
{{- if .Values.postgres.external.enabled }}
{{- .Values.postgres.external.connectionString }}
{{- else }}
{{- printf "postgres://%s:%s@%s:%d/%s?sslmode=%s" .Values.postgres.auth.username .Values.postgres.auth.password (printf "%s-postgres" (include "contextops.fullname" .)) (.Values.postgres.service.port | int) .Values.postgres.auth.database .Values.postgres.sslMode }}
{{- end }}
{{- end }}

{{/*
RabbitMQ URL
*/}}
{{- define "contextops.rabbitmqUrl" -}}
{{- if .Values.rabbitmq.external.enabled }}
{{- .Values.rabbitmq.external.connectionString }}
{{- else }}
{{- printf "amqp://%s:%s@%s:%d%s" .Values.rabbitmq.auth.username .Values.rabbitmq.auth.password (printf "%s-rabbitmq" (include "contextops.fullname" .)) (.Values.rabbitmq.service.port | int) .Values.rabbitmq.auth.vhost }}
{{- end }}
{{- end }}
{{/*
Base name for all resources. Defaults to the release name; override with
fullnameOverride. Truncated to 63 chars for label/DNS safety.
*/}}
{{- define "platformctl.fullname" -}}
{{- $name := default .Release.Name .Values.fullnameOverride -}}
{{- $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels applied to every object.
*/}}
{{- define "platformctl.labels" -}}
app.kubernetes.io/part-of: platformctl-platform
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end -}}

{{/*
Resolve the DATABASE_URL secret name for a merged service definition.
Usage: include "platformctl.dbSecretName" (dict "root" $root "svc" $merged)
*/}}
{{- define "platformctl.dbSecretName" -}}
{{- $root := .root -}}
{{- if eq (.svc.dbSecret | default "pool") "direct" -}}
{{- $root.Values.secretNames.direct -}}
{{- else -}}
{{- $root.Values.secretNames.pool -}}
{{- end -}}
{{- end -}}

{{/*
Merge a single service's values over the shared defaults (service wins).
Usage: include is awkward for map returns, so templates do this inline:
  {{- $merged := merge (deepCopy $svc) $root.Values.defaults -}}
This helper only documents the convention.
*/}}

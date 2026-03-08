{{/*
Expand the name of the chart.
*/}}
{{- define "agent-deployed-app.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "agent-deployed-app.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s" $name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "agent-deployed-app.labels" -}}
helm.sh/chart: {{ include "agent-deployed-app.name" . }}
{{ include "agent-deployed-app.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
moses.ai/tenant-id: {{ .Values.moses.tenantId | quote }}
moses.ai/execution-id: {{ .Values.moses.executionId | quote }}
{{- if .Values.moses.chartId }}
moses.ai/chart-id: {{ .Values.moses.chartId | quote }}
{{- end }}
{{- if .Values.moses.appSlug }}
moses.ai/app-slug: {{ .Values.moses.appSlug | quote }}
{{- end }}
moses.ai/managed-by: moses
{{- end }}

{{/*
Selector labels
*/}}
{{- define "agent-deployed-app.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agent-deployed-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Database host - uses in-cluster postgresql or external
*/}}
{{- define "showcase.databaseHost" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else }}
{{- .Values.externalDatabase.host }}
{{- end }}
{{- end }}

{{/*
Database port
*/}}
{{- define "showcase.databasePort" -}}
{{- if .Values.postgresql.enabled }}
{{- "5432" }}
{{- else }}
{{- .Values.externalDatabase.port | default "5432" }}
{{- end }}
{{- end }}

{{/*
Database user
*/}}
{{- define "showcase.databaseUser" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.username }}
{{- else }}
{{- .Values.externalDatabase.user }}
{{- end }}
{{- end }}

{{/*
Database name
*/}}
{{- define "showcase.databaseName" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.database }}
{{- else }}
{{- .Values.externalDatabase.database }}
{{- end }}
{{- end }}

{{/*
Database SSL mode
*/}}
{{- define "showcase.databaseSSLMode" -}}
{{- if .Values.postgresql.enabled }}
{{- "disable" }}
{{- else }}
{{- .Values.externalDatabase.sslmode | default "disable" }}
{{- end }}
{{- end }}

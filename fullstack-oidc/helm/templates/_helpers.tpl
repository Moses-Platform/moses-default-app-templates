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
moses-manager.eu/tenant-id: {{ .Values.moses.tenantId | quote }}
moses-manager.eu/execution-id: {{ .Values.moses.executionId | quote }}
{{- if .Values.moses.chartId }}
moses-manager.eu/chart-id: {{ .Values.moses.chartId | quote }}
{{- end }}
{{- if .Values.moses.appSlug }}
moses-manager.eu/app-slug: {{ .Values.moses.appSlug | quote }}
{{- end }}
moses-manager.eu/managed-by: moses
{{- end }}

{{/*
Selector labels
*/}}
{{- define "agent-deployed-app.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agent-deployed-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}


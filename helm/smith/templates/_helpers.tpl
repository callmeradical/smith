{{- define "smith.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "smith.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "smith.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "smith.labels" -}}
app.kubernetes.io/name: {{ include "smith.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "smith.selectorLabels" -}}
app.kubernetes.io/name: {{ include "smith.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "smith.serviceAccountName" -}}
{{- $component := .component -}}
{{- $root := .root -}}
{{- if (index $root.Values $component).serviceAccount.create -}}
{{- default (printf "%s-%s" (include "smith.fullname" $root) $component) (index (index $root.Values $component).serviceAccount "name") -}}
{{- else -}}
{{- default "default" (index (index $root.Values $component).serviceAccount "name") -}}
{{- end -}}
{{- end -}}

{{- define "smith.runtimeSecretName" -}}
{{- if .Values.secrets.existingSecret -}}
{{- .Values.secrets.existingSecret -}}
{{- else if .Values.secrets.create -}}
{{- printf "%s-runtime" (include "smith.fullname" .) -}}
{{- end -}}
{{- end -}}

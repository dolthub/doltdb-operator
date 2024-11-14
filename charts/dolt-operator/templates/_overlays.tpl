{{/*
Overlays will be merged with controller-gen generated yaml.
*/}}


{{- define "dolt-operator.clusterRoleOverlay" -}}
metadata:
  name: {{ include "dolt-operator.fullname" . }}
  labels:
    {{- include "dolt-operator.labels" . | nindent 4 }}
{{- end }}

{{- define "dolt-operator.crdOverlay" -}}
metadata:
  labels:
    {{- include "dolt-operator.labels" . | nindent 4 }}
{{- if .Values.keepCrds }}
  annotations:
    helm.sh/resource-policy: keep
{{- end }}
{{- end }}

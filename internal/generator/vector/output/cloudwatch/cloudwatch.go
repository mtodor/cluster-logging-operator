package cloudwatch

import (
	"fmt"
	logging "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	. "github.com/openshift/cluster-logging-operator/internal/generator"
	genhelper "github.com/openshift/cluster-logging-operator/internal/generator/helpers"
	. "github.com/openshift/cluster-logging-operator/internal/generator/vector/elements"
	"github.com/openshift/cluster-logging-operator/internal/generator/vector/helpers"
	"github.com/openshift/cluster-logging-operator/internal/generator/vector/output/security"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

type Endpoint struct {
	URL string
}

func (e Endpoint) Name() string {
	return "awsEndpointTemplate"
}

func (e Endpoint) Template() (ret string) {
	ret = `{{define "` + e.Name() + `" -}}`
	if e.URL != "" {
		ret += `endpoint = "{{ .URL }}"
tls.verify_certificate = false`
	}
	ret += `{{end}}`
	return
}

type CloudWatch struct {
	Desc           string
	ComponentID    string
	Inputs         string
	Region         string
	EndpointConfig Element
	SecurityConfig Element
}

func (e CloudWatch) Name() string {
	return "cloudwatchTemplate"
}

func (e CloudWatch) Template() string {

	return `{{define "` + e.Name() + `" -}}
{{if .Desc -}}
# {{.Desc}}
{{end -}}
[sinks.{{.ComponentID}}]
type = "aws_cloudwatch_logs"
inputs = {{.Inputs}}
region = "{{.Region}}"
compression = "none"
group_name = "{{"{{ group_name }}"}}"
stream_name = "{{"{{ stream_name }}"}}"
{{compose_one .SecurityConfig}}
encoding.codec = "json"
request.concurrency = 2
{{compose_one .EndpointConfig}}
{{- end}}
`
}

func Conf(o logging.OutputSpec, inputs []string, secret *corev1.Secret, op Options) []Element {
	outputName := helpers.FormatComponentID(o.Name)
	componentID := fmt.Sprintf("%s_%s", outputName, "normalize_group_and_streams")
	if genhelper.IsDebugOutput(op) {
		return []Element{
			NormalizeGroupAndStreamName(LogGroupNameField(o), LogGroupPrefix(o), componentID, inputs),
			Debug(outputName, helpers.MakeInputs([]string{componentID}...)),
		}
	}
	return []Element{
		NormalizeGroupAndStreamName(LogGroupNameField(o), LogGroupPrefix(o), componentID, inputs),
		OutputConf(o, []string{componentID}, secret, op, o.Cloudwatch.Region),
	}
}

func OutputConf(o logging.OutputSpec, inputs []string, secret *corev1.Secret, op Options, region string) Element {
	return CloudWatch{
		Desc:           "Cloudwatch Logs",
		ComponentID:    helpers.FormatComponentID(o.Name),
		Inputs:         helpers.MakeInputs(inputs...),
		Region:         region,
		SecurityConfig: SecurityConfig(secret),
		EndpointConfig: EndpointConfig(o),
	}
}

func SecurityConfig(secret *corev1.Secret) Element {
	return AWSKey{
		KeyID:     strings.TrimSpace(security.GetFromSecret(secret, "aws_access_key_id")),
		KeySecret: strings.TrimSpace(security.GetFromSecret(secret, "aws_secret_access_key")),
	}
}

func EndpointConfig(o logging.OutputSpec) Element {
	return Endpoint{
		URL: o.URL,
	}
}

func NormalizeGroupAndStreamName(logGroupNameField string, logGroupPrefix string, componentID string, inputs []string) Element {
	appGroupName := fmt.Sprintf("%q + %s", logGroupPrefix, logGroupNameField)
	auditGroupName := fmt.Sprintf("%s%s", logGroupPrefix, "audit")
	infraGroupName := fmt.Sprintf("%s%s", logGroupPrefix, "infrastructure")
	vrl := strings.TrimSpace(`
.group_name = "default"
.stream_name = "default"

if (.file != null) {
 .file = "kubernetes" + replace!(.file, "/", ".")
 .stream_name = del(.file)
}

if ( .log_type == "application" ) {
 .group_name = ( ` + appGroupName + ` ) ?? "application"
}
if ( .log_type == "audit" ) {
 .group_name = "` + auditGroupName + `"
 .stream_name = ( "${VECTOR_SELF_NODE_NAME}" + .tag ) ?? .stream_name
}
if ( .log_type == "infrastructure" ) {
 .group_name = "` + infraGroupName + `"
 .stream_name = ( .hostname + "." + .stream_name ) ?? .stream_name
}
if ( .tag == ".journal.system" ) {
 .stream_name =  ( .hostname + .tag ) ?? .stream_name
}
del(.tag)
del(.source_type)
	`)
	return Remap{
		Desc:        "Cloudwatch Group and Stream Names",
		ComponentID: componentID,
		Inputs:      helpers.MakeInputs(inputs...),
		VRL:         vrl,
	}
}

func LogGroupPrefix(o logging.OutputSpec) string {
	if o.Cloudwatch != nil {
		prefix := o.Cloudwatch.GroupPrefix
		if prefix != nil && strings.TrimSpace(*prefix) != "" {
			return fmt.Sprintf("%s.", *prefix)
		}
	}
	return ""
}

// LogGroupNameField Return the field used for grouping the application logs
func LogGroupNameField(o logging.OutputSpec) string {
	if o.Cloudwatch != nil {
		switch o.Cloudwatch.GroupBy {
		case logging.LogGroupByNamespaceName:
			return ".kubernetes.namespace_name"
		case logging.LogGroupByNamespaceUUID:
			return ".kubernetes.namespace_uid"
		default:
			return ".log_type"
		}
	}
	return ""
}

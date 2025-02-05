package elasticsearchunmanaged

import (
	"fmt"
	"github.com/openshift/cluster-logging-operator/internal/constants"
	"github.com/openshift/cluster-logging-operator/test/framework/functional"
	"path/filepath"
	"runtime"

	framework "github.com/openshift/cluster-logging-operator/test/framework/e2e"
	testruntime "github.com/openshift/cluster-logging-operator/test/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	log "github.com/ViaQ/logerr/v2/log/static"
	logging "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/openshift/cluster-logging-operator/test/helpers"
	elasticsearch "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

var _ = Describe("[ClusterLogForwarder] Forwards logs", func() {

	_, filename, _, _ := runtime.Caller(0)
	log.Info("Running ", "filename", filename)
	var (
		err            error
		e2e            = framework.NewE2ETestFramework()
		pipelineSecret *corev1.Secret
		elasticsearch  *elasticsearch.Elasticsearch
	)

	BeforeEach(func() {
		rootDir := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "/")
		log.V(3).Info("Repo ", "rootDir", rootDir)
		err = e2e.DeployLogGenerator()
		if err != nil {
			Fail(fmt.Sprintf("Unable to deploy log generator. E: %s", err.Error()))
		}

		if elasticsearch, pipelineSecret, err = e2e.DeployAnElasticsearchCluster(rootDir); err != nil {
			Fail(fmt.Sprintf("Unable to deploy an elastic instance: %v", err))
		}

		cr := helpers.NewClusterLogging(helpers.ComponentTypeCollector)
		if err := e2e.CreateClusterLogging(cr); err != nil {
			Fail(fmt.Sprintf("Unable to create an instance of cluster logging: %v", err))
		}
	})

	Describe("when the output is Elasticsearch", func() {

		Context("and JSON parsing is enabled", func() {

			It("should fail validation when structuredTypeKey references an invalid value", func() {
				forwarder := testruntime.NewClusterLogForwarder()
				clfb := functional.NewClusterLogForwarderBuilder(forwarder).
					FromInput(logging.InputNameApplication).
					ToOutputWithVisitor(func(spec *logging.OutputSpec) {
						spec.Elasticsearch = &logging.Elasticsearch{
							StructuredTypeKey: "junk",
						}
					}, logging.OutputTypeElasticsearch)
				clfb.Forwarder.Spec.Pipelines[0].Parse = "json"
				var err error
				if err = e2e.CreateClusterLogForwarder(forwarder); err == nil {
					Fail(fmt.Sprintf("Expected kubevalidation to fail creation of: %#v", forwarder))
				}
				Expect(err.Error()).To(MatchRegexp(`invalid.*spec\.outputs\[[0-9]*\]\.elasticsearch\.structuredTypeKey`))
			})

		})

	})

	Describe("when the output is a third-party managed elasticsearch", func() {

		BeforeEach(func() {
			forwarder := &logging.ClusterLogForwarder{
				TypeMeta: metav1.TypeMeta{
					Kind:       logging.ClusterLogForwarderKind,
					APIVersion: logging.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "instance",
				},
				Spec: logging.ClusterLogForwarderSpec{
					Outputs: []logging.OutputSpec{
						{
							Name: elasticsearch.Name,
							Secret: &logging.OutputSecretSpec{
								Name: pipelineSecret.ObjectMeta.Name,
							},
							Type: logging.OutputTypeElasticsearch,
							URL:  fmt.Sprintf("https://%s.%s.svc:9200", elasticsearch.Name, elasticsearch.Namespace),
						},
					},
					Pipelines: []logging.PipelineSpec{
						{
							Name:       "test-app",
							OutputRefs: []string{elasticsearch.Name},
							InputRefs:  []string{logging.InputNameApplication},
						},
						{
							Name:       "test-infra",
							OutputRefs: []string{elasticsearch.Name},
							InputRefs:  []string{logging.InputNameInfrastructure},
						},
						{
							Name:       "test-audit",
							OutputRefs: []string{elasticsearch.Name},
							InputRefs:  []string{logging.InputNameAudit},
						},
					},
				},
			}
			if err := e2e.CreateClusterLogForwarder(forwarder); err != nil {
				Fail(fmt.Sprintf("Unable to create an instance of clusterlogforwarder: %v", err))
			}

			components := []helpers.LogComponentType{helpers.ComponentTypeCollector, helpers.ComponentTypeStore}
			for _, component := range components {
				if err := e2e.WaitFor(component); err != nil {
					Fail(fmt.Sprintf("Failed waiting for component %s to be ready: %v", component, err))
				}
			}
		})

		It("should send logs to the forward.Output logstore", func() {
			name := elasticsearch.GetName()
			Expect(e2e.LogStores[name].HasInfraStructureLogs(framework.DefaultWaitForLogsTimeout)).To(BeTrue(), "Expected to find stored infrastructure logs")
			Expect(e2e.LogStores[name].HasApplicationLogs(framework.DefaultWaitForLogsTimeout)).To(BeTrue(), "Expected to find stored application logs")
			Expect(e2e.LogStores[name].HasAuditLogs(framework.DefaultWaitForLogsTimeout)).To(BeTrue(), "Expected to find stored audit logs")
		})

	})

	AfterEach(func() {
		e2e.Cleanup()
		e2e.WaitForCleanupCompletion(constants.OpenshiftNS, []string{constants.CollectorName, "elasticsearch"})
	})
})

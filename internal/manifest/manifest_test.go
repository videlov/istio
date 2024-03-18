package manifest

import (
	"os"
	"path"
	"testing"

	"github.com/kyma-project/istio/operator/internal/tests"
	"github.com/onsi/ginkgo/v2/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/istio/operator/api/v1alpha2"
	"github.com/kyma-project/istio/operator/internal/clusterconfig"
	iopv1alpha1 "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manifest Suite")
}

var _ = ReportAfterSuite("custom reporter", func(report types.Report) {
	tests.GenerateGinkgoJunitReport("manifest-suite", report)
})

var _ = Describe("Manifest merge", func() {
	numTrustedProxies := 4
	istioCR := &v1alpha2.Istio{ObjectMeta: metav1.ObjectMeta{
		Name:      "istio-test",
		Namespace: "namespace",
	},
		Spec: v1alpha2.IstioSpec{
			Config: v1alpha2.Config{
				NumTrustedProxies: &numTrustedProxies,
			},
		},
	}
	workingDir := "test"

	It("should return error when provided invalid cluster size", func() {
		// given
		sut := IstioMerger{workingDir}

		// when
		mergedIstioOperatorPath, err := sut.Merge(9, istioCR, clusterconfig.ClusterConfiguration{})

		// then
		Expect(err).Should(HaveOccurred())
		Expect(mergedIstioOperatorPath).To(BeEmpty())
	})

	It("should return merged configuration, when there is a Istio CR with valid configuration and using production manifest", func() {
		// given
		sut := NewDefaultIstioMerger()

		// when
		mergedIstioOperatorPath, err := sut.Merge(clusterconfig.Production, istioCR, clusterconfig.ClusterConfiguration{})

		// then
		Expect(err).Should(Not(HaveOccurred()))
		Expect(mergedIstioOperatorPath).To(Not(BeEmpty()))
	})

	It("should return error when provided misconfigured default Istio Operator", func() {
		// given
		sut := IstioMerger{workingDir}
		wrongOperator, err := os.ReadFile("test/wrong-operator.yaml")
		productionOperator = wrongOperator
		Expect(err).Should(Not(HaveOccurred()))
		Expect(productionOperator).To(Not(BeEmpty()))

		// when
		mergedIstioOperatorPath, err := sut.Merge(clusterconfig.Production, istioCR, clusterconfig.ClusterConfiguration{})

		// then
		Expect(err).Should(HaveOccurred())
		Expect(mergedIstioOperatorPath).To(BeEmpty())
	})

	It("should return merged configuration, when there is a Istio CR with valid configuration and a correct Istio Operator manifest", func() {
		// given
		sut := IstioMerger{workingDir}
		goodOperator, err := os.ReadFile("test/test-operator.yaml")
		productionOperator = goodOperator
		Expect(err).Should(Not(HaveOccurred()))
		Expect(productionOperator).To(Not(BeEmpty()))

		// when
		mergedIstioOperatorPath, err := sut.Merge(clusterconfig.Production, istioCR, clusterconfig.ClusterConfiguration{})

		// then
		Expect(err).ShouldNot(HaveOccurred())
		Expect(mergedIstioOperatorPath).To(Equal(path.Join(workingDir, mergedIstioOperatorFile)))

		iop := readIOP(mergedIstioOperatorPath)

		numTrustedProxies := iop.Spec.MeshConfig.Fields["defaultConfig"].
			GetStructValue().Fields["gatewayTopology"].GetStructValue().Fields["numTrustedProxies"].GetNumberValue()

		Expect(numTrustedProxies).To(Equal(float64(4)))
		err = os.Remove(mergedIstioOperatorPath)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("should return merged configuration with overrides when provided", func() {
		// given
		newCniBinDirPath := "overriden/path"

		clusterConfig := map[string]interface{}{
			"spec": map[string]interface{}{
				"components": map[string]interface{}{
					"base": map[string]bool{
						"enabled": false,
					},
				},
				"values": map[string]interface{}{
					"cni": map[string]string{
						"cniBinDir": newCniBinDirPath,
					},
				},
			},
		}

		sut := IstioMerger{workingDir}
		ProductionOperator, err := os.ReadFile("test/test-operator.yaml")
		Expect(err).Should(Not(HaveOccurred()))
		Expect(ProductionOperator).To(Not(BeEmpty()))

		// when
		mergedIstioOperatorPath, err := sut.Merge(clusterconfig.Production, istioCR, clusterConfig)

		// then
		Expect(err).ShouldNot(HaveOccurred())
		Expect(mergedIstioOperatorPath).To(Equal(path.Join(workingDir, mergedIstioOperatorFile)))

		iop := readIOP(mergedIstioOperatorPath)

		numTrustedProxies := iop.Spec.MeshConfig.Fields["defaultConfig"].
			GetStructValue().Fields["gatewayTopology"].GetStructValue().Fields["numTrustedProxies"].GetNumberValue()

		Expect(numTrustedProxies).To(Equal(float64(4)))

		baseEnabled := iop.Spec.Components.Base.Enabled.Value
		Expect(baseEnabled).To(BeFalse())

		Expect(iop.Spec.Values.Fields["cni"]).NotTo(BeNil())
		Expect(iop.Spec.Values.Fields["cni"].GetStructValue().Fields["cniBinDir"]).NotTo(BeNil())
		cniBinDir := iop.Spec.Values.Fields["cni"].GetStructValue().Fields["cniBinDir"].GetStringValue()
		Expect(cniBinDir).To(Equal(newCniBinDirPath))

		err = os.Remove(mergedIstioOperatorPath)
		Expect(err).ShouldNot(HaveOccurred())
	})
})

func readIOP(iopv1alpha1FilePath string) iopv1alpha1.IstioOperator {
	iop := iopv1alpha1.IstioOperator{}

	manifest, err := os.ReadFile(iopv1alpha1FilePath)
	Expect(err).ShouldNot(HaveOccurred())

	err = yaml.Unmarshal(manifest, &iop)
	Expect(err).ShouldNot(HaveOccurred())

	return iop
}

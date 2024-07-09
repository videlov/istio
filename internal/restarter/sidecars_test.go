package restarter_test

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	operatorv1alpha2 "github.com/kyma-project/istio/operator/api/v1alpha2"
	"github.com/kyma-project/istio/operator/internal/clusterconfig"
	"github.com/kyma-project/istio/operator/internal/described_errors"
	"github.com/kyma-project/istio/operator/internal/filter"
	"github.com/kyma-project/istio/operator/internal/istiooperator"
	"github.com/kyma-project/istio/operator/internal/restarter"
	"github.com/kyma-project/istio/operator/internal/status"
	"github.com/kyma-project/istio/operator/pkg/lib/gatherer"
	"github.com/kyma-project/istio/operator/pkg/lib/sidecars"
	"github.com/kyma-project/istio/operator/pkg/lib/sidecars/pods"
	"github.com/kyma-project/istio/operator/pkg/lib/sidecars/restart"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"
	networkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	iopv1alpha1 "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

var _ = Describe("SidecarsRestarter reconciliation", func() {
	It("should fail proxy reset if Istio pods do not match target version", func() {
		// given
		numTrustedProxies := 1
		istioCr := operatorv1alpha2.Istio{ObjectMeta: metav1.ObjectMeta{
			Name:            "default",
			ResourceVersion: "1",
			Annotations:     map[string]string{},
		},
			Spec: operatorv1alpha2.IstioSpec{
				Config: operatorv1alpha2.Config{
					NumTrustedProxies: &numTrustedProxies,
				},
			},
		}
		istiod := createPod("istiod", gatherer.IstioNamespace, "discovery", "1.16.0")
		fakeClient := createFakeClient(&istioCr, istiod)
		statusHandler := status.NewStatusHandler(fakeClient)
		sidecarsRestarter := restarter.NewSidecarsRestarter(logr.Discard(), createFakeClient(&istioCr, istiod),
			&MergerMock{"1.16.1-distroless"}, sidecars.NewProxyResetter(), statusHandler)
		// when
		err, requeue := sidecarsRestarter.Restart(context.Background(), &istioCr)

		// then
		Expect(err).Should(HaveOccurred())
		Expect(err.Level()).To(Equal(described_errors.Error))
		Expect(err.Error()).To(ContainSubstring("istio-system Pods version 1.16.0 do not match istio operator version 1.16.1"))
		Expect(requeue).To(BeFalse())
		Expect((*istioCr.Status.Conditions)[0].Message).To(Equal("Proxy sidecar restart failed"))
	})

	It("should succeed proxy reset even if more than 5 proxies could not be reset and will return a warning", func() {
		// given
		numTrustedProxies := 1
		istioCr := operatorv1alpha2.Istio{ObjectMeta: metav1.ObjectMeta{
			Name:            "default",
			ResourceVersion: "1",
			Annotations:     map[string]string{},
		},
			Spec: operatorv1alpha2.IstioSpec{
				Config: operatorv1alpha2.Config{
					NumTrustedProxies: &numTrustedProxies,
				},
			},
		}
		istiod := createPod("istiod", gatherer.IstioNamespace, "discovery", "1.16.1")
		proxyResetter := &proxyResetterMock{
			restartWarnings: []restart.RestartWarning{
				{
					Name:      "name1",
					Namespace: "ns1",
				},
				{
					Name:      "name2",
					Namespace: "ns2",
				},
				{
					Name:      "name3",
					Namespace: "ns3",
				},
				{
					Name:      "name4",
					Namespace: "ns4",
				},
				{
					Name:      "name5",
					Namespace: "ns5",
				},
				{
					Name:      "name6",
					Namespace: "ns6",
				},
			},
		}
		fakeClient := createFakeClient(&istioCr, istiod)
		statusHandler := status.NewStatusHandler(fakeClient)
		sidecarsRestarter := restarter.NewSidecarsRestarter(logr.Discard(), createFakeClient(&istioCr, istiod),
			&MergerMock{"1.16.1-distroless"}, proxyResetter, statusHandler)

		// when
		err, requeue := sidecarsRestarter.Restart(context.Background(), &istioCr)

		// then
		Expect(err).Should(HaveOccurred())
		Expect(err.Level()).To(Equal(described_errors.Warning))
		Expect(requeue).To(BeFalse())
		Expect((*istioCr.Status.Conditions)[0].Message).To(ContainSubstring("The sidecars of the following workloads could not be restarted: ns1/name1, ns2/name2, ns3/name3, ns4/name4, ns5/name5 and 1 additional workload(s)"))
	})

	It("should succeed proxy reset even if less than 5 proxies could not be reset and will return a warning", func() {
		// given
		numTrustedProxies := 1
		istioCr := operatorv1alpha2.Istio{ObjectMeta: metav1.ObjectMeta{
			Name:            "default",
			ResourceVersion: "1",
			Annotations:     map[string]string{},
		},
			Spec: operatorv1alpha2.IstioSpec{
				Config: operatorv1alpha2.Config{
					NumTrustedProxies: &numTrustedProxies,
				},
			},
		}
		istiod := createPod("istiod", gatherer.IstioNamespace, "discovery", "1.16.1")
		proxyResetter := &proxyResetterMock{
			restartWarnings: []restart.RestartWarning{
				{
					Name:      "name1",
					Namespace: "ns1",
				},
				{
					Name:      "name2",
					Namespace: "ns2",
				},
			},
		}
		fakeClient := createFakeClient(&istioCr, istiod)
		statusHandler := status.NewStatusHandler(fakeClient)
		sidecarsRestarter := restarter.NewSidecarsRestarter(logr.Discard(), createFakeClient(&istioCr, istiod),
			&MergerMock{"1.16.1-distroless"}, proxyResetter, statusHandler)

		// when
		err, requeue := sidecarsRestarter.Restart(context.Background(), &istioCr)

		// then
		Expect(err).Should(HaveOccurred())
		Expect(err.Level()).To(Equal(described_errors.Warning))
		Expect(requeue).To(BeFalse())
		Expect((*istioCr.Status.Conditions)[0].Message).To(Equal("The sidecars of the following workloads could not be restarted: ns1/name1, ns2/name2"))
	})

	It("should succeed proxy reset when there is no warning or errors", func() {
		// given
		numTrustedProxies := 1
		istioCr := operatorv1alpha2.Istio{ObjectMeta: metav1.ObjectMeta{
			Name:            "default",
			ResourceVersion: "1",
			Annotations:     map[string]string{},
		},
			Spec: operatorv1alpha2.IstioSpec{
				Config: operatorv1alpha2.Config{
					NumTrustedProxies: &numTrustedProxies,
				},
			},
		}
		istiod := createPod("istiod", gatherer.IstioNamespace, "discovery", "1.16.1")
		proxyResetter := &proxyResetterMock{}
		fakeClient := createFakeClient(&istioCr, istiod)
		statusHandler := status.NewStatusHandler(fakeClient)
		sidecarsRestarter := restarter.NewSidecarsRestarter(logr.Discard(), createFakeClient(&istioCr, istiod),
			&MergerMock{"1.16.1-distroless"}, proxyResetter, statusHandler)

		// when
		err, requeue := sidecarsRestarter.Restart(context.Background(), &istioCr)

		// then
		Expect(err).Should(Not(HaveOccurred()))
		Expect(requeue).To(BeFalse())
		Expect((*istioCr.Status.Conditions)[0].Reason).To(Equal(string(operatorv1alpha2.ConditionReasonProxySidecarRestartSucceeded)))
		Expect((*istioCr.Status.Conditions)[0].Message).To(Equal(operatorv1alpha2.ConditionReasonProxySidecarRestartSucceededMessage))
	})
})

func createFakeClient(objects ...client.Object) client.Client {
	err := operatorv1alpha2.AddToScheme(scheme.Scheme)
	Expect(err).ShouldNot(HaveOccurred())
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).ShouldNot(HaveOccurred())
	err = networkingv1.AddToScheme(scheme.Scheme)
	Expect(err).ShouldNot(HaveOccurred())

	return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objects...).Build()
}

func createPod(name, namespace, containerName, imageVersion string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: "image:" + imageVersion,
				},
			},
		},
	}
}

type MergerMock struct {
	tag string
}

func (m MergerMock) Merge(_ clusterconfig.ClusterSize, _ *operatorv1alpha2.Istio, _ clusterconfig.ClusterConfiguration) (string, error) {
	return "mocked istio operator merge result", nil
}

func (m MergerMock) GetIstioOperator(_ clusterconfig.ClusterSize) (iopv1alpha1.IstioOperator, error) {
	iop := iopv1alpha1.IstioOperator{}
	istioOperator, err := os.ReadFile("../../internal/istiooperator/istio-operator.yaml")
	if err == nil {
		err = yaml.Unmarshal(istioOperator, &iop)
	}
	iop.Spec.Tag = structpb.NewStringValue(m.tag)
	return iop, err
}

func (m MergerMock) GetIstioImageVersion() (istiooperator.IstioImageVersion, error) {
	return istiooperator.NewIstioImageVersionFromTag("1.16.1-distroless")
}

func (m MergerMock) SetIstioInstallFlavor(_ clusterconfig.ClusterSize) {}

type proxyResetterMock struct {
	restartWarnings []restart.RestartWarning
	hasMorePods     bool
	err             error
}

func (p *proxyResetterMock) ProxyReset(_ context.Context, _ client.Client, _ pods.SidecarImage, _ v1.ResourceRequirements, _ []filter.SidecarProxyPredicate, _ *logr.Logger) ([]restart.RestartWarning, bool, error) {
	return p.restartWarnings, p.hasMorePods, p.err
}

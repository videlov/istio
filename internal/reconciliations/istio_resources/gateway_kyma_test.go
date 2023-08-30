package istio_resources

import (
	"bytes"
	"context"
	"text/template"

	"github.com/kyma-project/istio/operator/internal/reconciliations/istio"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Apply", func() {
	It("should return created if no resource was present", func() {
		//given
		client := createFakeClient()
		sample := NewGatewayKyma(client)

		templateValues := map[string]string{}
		templateValues["DomainName"] = "example.com"

		//when
		changed, err := sample.apply(context.TODO(), client, templateValues)

		//then
		Expect(err).To(Not(HaveOccurred()))
		Expect(changed).To(Equal(controllerutil.OperationResultCreated))

		var s networkingv1alpha3.GatewayList
		listErr := client.List(context.TODO(), &s)
		Expect(listErr).To(Not(HaveOccurred()))
		Expect(s.Items).To(HaveLen(1))

		Expect(s.Items[0].Annotations).To(Not(BeNil()))
		Expect(s.Items[0].Annotations[istio.DisclaimerKey]).To(Not(BeNil()))
	})

	It("should return not changed if no change was applied", func() {
		//given
		resourceTemplate, err := template.New("tmpl").Option("missingkey=error").Parse(string(manifest_gateway_kyma))
		Expect(err).To(Not(HaveOccurred()))

		templateValues := map[string]string{}
		templateValues["DomainName"] = "example.com"

		var resourceBuffer bytes.Buffer
		err = resourceTemplate.Execute(&resourceBuffer, templateValues)
		Expect(err).To(Not(HaveOccurred()))

		var p networkingv1alpha3.Gateway
		err = yaml.Unmarshal(resourceBuffer.Bytes(), &p)
		Expect(err).To(Not(HaveOccurred()))

		client := createFakeClient(&p)

		sample := NewGatewayKyma(client)

		//when
		changed, err := sample.apply(context.TODO(), client, templateValues)

		//then
		Expect(err).To(Not(HaveOccurred()))
		Expect(changed).To(Equal(controllerutil.OperationResultNone))

		var s networkingv1alpha3.GatewayList
		listErr := client.List(context.TODO(), &s)
		Expect(listErr).To(Not(HaveOccurred()))
		Expect(s.Items).To(HaveLen(1))

		Expect(s.Items[0].Annotations).To(Not(BeNil()))
		Expect(s.Items[0].Annotations[istio.DisclaimerKey]).To(Not(BeNil()))
	})

	It("should return updated if change was applied", func() {
		//given
		resourceTemplate, err := template.New("tmpl").Option("missingkey=error").Parse(string(manifest_gateway_kyma))
		Expect(err).To(Not(HaveOccurred()))

		templateValues := map[string]string{}
		templateValues["DomainName"] = "example.com"

		var resourceBuffer bytes.Buffer
		err = resourceTemplate.Execute(&resourceBuffer, templateValues)
		Expect(err).To(Not(HaveOccurred()))

		var p networkingv1alpha3.Gateway
		err = yaml.Unmarshal(resourceBuffer.Bytes(), &p)
		Expect(err).To(Not(HaveOccurred()))

		p.Spec.Servers[0].Port.Number = 4443
		client := createFakeClient(&p)

		sample := NewGatewayKyma(client)

		//when
		changed, err := sample.apply(context.TODO(), client, templateValues)

		//then
		Expect(err).To(Not(HaveOccurred()))
		Expect(changed).To(Equal(controllerutil.OperationResultUpdated))

		var s networkingv1alpha3.GatewayList
		listErr := client.List(context.TODO(), &s)
		Expect(listErr).To(Not(HaveOccurred()))
		Expect(s.Items).To(HaveLen(1))

		Expect(s.Items[0].Annotations).To(Not(BeNil()))
		Expect(s.Items[0].Annotations[istio.DisclaimerKey]).To(Not(BeNil()))
	})
})
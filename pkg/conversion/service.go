package conversion

import (
	"context"
	"encoding/json"

	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving/v1beta1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConvertToService(deployment *appsv1.Deployment) (*v1alpha1.Service, error) {
	// Remove benign container names as they won't be needed anyway
	for i := range deployment.Spec.Template.Spec.Containers {
		deployment.Spec.Template.Spec.Containers[i].Name = ""
	}

	specJSON, err := json.Marshal(deployment.Spec.Template.Spec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal deployment into json")
	}

	rev := &v1beta1.PodSpec{}
	if err := json.Unmarshal(specJSON, rev); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshall json into revision podspec")
	}

	if err := rev.Validate(context.Background()); err != nil {
		return nil, errors.Wrap(err, "deployment does not qualify as a revision")
	}

	// construct Knative Service
	return &v1alpha1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "serving.knative.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		},
		Spec: v1alpha1.ServiceSpec{
			ConfigurationSpec: v1alpha1.ConfigurationSpec{
				Template: &v1alpha1.RevisionTemplateSpec{
					Spec: v1alpha1.RevisionSpec{
						RevisionSpec: v1beta1.RevisionSpec{
							PodSpec: *rev,
						},
					},
				},
			},
		},
	}, nil
}

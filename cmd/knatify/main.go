package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func main() {
	filename := flag.String("filename", "", "file to check and transform")
	flag.Parse()

	if *filename == "" {
		fmt.Println("--filename must be set")
		os.Exit(1)
	}

	reader, err := os.Open(*filename)
	if err != nil {
		fmt.Println("Failed to open file:", err)
		os.Exit(1)
	}
	defer reader.Close()

	decoder := yaml.NewYAMLToJSONDecoder(reader)
	deployment := &appsv1.Deployment{}
	if err := decoder.Decode(deployment); err != nil {
		fmt.Println("Failed to parse deployment:", err)
		os.Exit(1)
	}

	// Remove benign container names as they won't be needed anyway
	for i := range deployment.Spec.Template.Spec.Containers {
		deployment.Spec.Template.Spec.Containers[i].Name = ""
	}

	specJSON, err := json.Marshal(deployment.Spec.Template.Spec)
	if err != nil {
		fmt.Println("Failed to marshal deployment into json:", err)
		os.Exit(1)
	}

	rev := &v1beta1.PodSpec{}
	if err := json.Unmarshal(specJSON, rev); err != nil {
		fmt.Println("Failed to unmarshall json into revision podspec:", err)
		os.Exit(1)
	}

	if err := rev.Validate(context.Background()); err != nil {
		fmt.Println("Deployment does not qualify as a Revision:", err)
		os.Exit(1)
	}

	// construct Knative Service
	ksvc := &v1alpha1.Service{
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
	}

	bytes, _ := json.Marshal(ksvc)

	fmt.Println(string(bytes))
}

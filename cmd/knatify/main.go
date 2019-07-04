package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/markusthoemmes/knatify/pkg/conversion"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	decoder := yaml.NewYAMLToJSONDecoder(reader)
	deployment := &appsv1.Deployment{}
	if err := decoder.Decode(deployment); err != nil {
		fmt.Println("Failed to parse deployment:", err)
		os.Exit(1)
	}

	ksvc, err := conversion.ConvertToService(deployment)
	if err != nil {
		fmt.Println("Failed to convert deployment to service:", err)
		os.Exit(1)
	}

	bytes, err := json.Marshal(ksvc)
	if err != nil {
		fmt.Println("Failed to marshal service to json:", err)
		os.Exit(1)
	}

	fmt.Println(string(bytes))
}

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/knative/client/pkg/wait"
	"github.com/knative/pkg/apis"
	serving_v1alpha1_api "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	serving "github.com/knative/serving/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	"github.com/markusthoemmes/knatify/pkg/conversion"
	route_v1_api "github.com/openshift/api/route/v1"
	route "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	clientCfg, err := config.ClientConfig()
	if err != nil {
		panic(err.Error())
	}

	namespaceCfg, _, err := config.Namespace()
	if err != nil {
		panic(err.Error())
	}

	var (
		namespace      string
		routeName      string
		deploymentName string
	)

	flag.StringVar(&namespace, "namespace", namespaceCfg, "")
	flag.StringVar(&routeName, "route", "", "")
	flag.StringVar(&deploymentName, "deployment", "", "")
	flag.Parse()

	proxyServiceName := "istio-ingressgateway-proxy"

	kube := kubernetes.NewForConfigOrDie(clientCfg)
	serving := serving.NewForConfigOrDie(clientCfg)
	ocp := route.NewForConfigOrDie(clientCfg)

	deployments := kube.AppsV1().Deployments(namespace)
	services := serving.Services(namespace)
	routes := ocp.Routes(namespace)

	fmt.Printf("Fetching deployment '%s' ... ", deploymentName)
	deployment, err := deployments.Get(deploymentName, metav1.GetOptions{})
	failIfError(err)

	fmt.Print("Converting deployment to Knative Service ... ")
	ksvc, err := conversion.ConvertToService(deployment)
	failIfError(err)

	fmt.Printf("Creating Knative Service '%s' ... ", ksvc.Name)
	_, err = services.Create(ksvc)
	failIfError(err)

	waiter := wait.NewWaitForReady("service", services.Watch, func(obj runtime.Object) (apis.Conditions, error) {
		service := obj.(*serving_v1alpha1_api.Service)
		return apis.Conditions(service.Status.Conditions), nil
	})

	fmt.Printf("Waiting for Knative Service '%s' to become ready ... ", ksvc.Name)
	err = waiter.Wait(ksvc.Name, 10*time.Minute, ioutil.Discard)
	failIfError(err)

	fmt.Printf("Fetching Openshift route '%s' ... ", routeName)
	route, err := routes.Get(routeName, metav1.GetOptions{})
	failIfError(err)

	fmt.Printf("Cutting over to Knative Service ... ")
	for i := 0; i <= 10; i++ {
		weight := int32(i * 10)
		oldWeight := 100 - weight

		fmt.Printf("\rCutting over to service ... %d%% ... ", weight)

		route.Spec.To.Weight = &oldWeight

		ksvcBackend := route_v1_api.RouteTargetReference{
			Kind:   "Service",
			Name:   proxyServiceName,
			Weight: &weight,
		}
		route.Spec.AlternateBackends = []route_v1_api.RouteTargetReference{ksvcBackend}

		route, err = routes.Update(route)
		if err != nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
	failIfError(err)

	fmt.Println()
	color.Blue("Welcome to the serverless world!")
}

func failIfError(err error) {
	if err != nil {
		color.Red("FAILED: %v", err)
		os.Exit(1)
	}
	color.Green("OK")
}

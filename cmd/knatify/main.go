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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		namespace        string
		routeName        string
		deploymentName   string
		proxyServiceName string = "istio-ingressgateway-proxy"

		// TODO: Do not hardcode these, read them from Knative config.
		istioNamespace   string = "istio-system"
		istioGatewayName string = "istio-ingressgateway"
	)

	flag.StringVar(&namespace, "namespace", namespaceCfg, "")
	flag.StringVar(&routeName, "route", "", "")
	flag.StringVar(&deploymentName, "deployment", "", "")
	flag.Parse()

	kube := kubernetes.NewForConfigOrDie(clientCfg)
	serving := serving.NewForConfigOrDie(clientCfg)
	ocp := route.NewForConfigOrDie(clientCfg)

	deployments := kube.AppsV1().Deployments(namespace)
	k8sServices := kube.CoreV1().Services(namespace)
	endpoints := kube.CoreV1().Endpoints(namespace)
	knServices := serving.Services(namespace)
	routes := ocp.Routes(namespace)

	// Basic validations
	fmt.Printf("Fetching deployment '%s' ... ", deploymentName)
	deployment, err := deployments.Get(deploymentName, metav1.GetOptions{})
	failIfError(err)

	fmt.Printf("Fetching Openshift route '%s' ... ", routeName)
	route, err := routes.Get(routeName, metav1.GetOptions{})
	failIfError(err)

	// Setting up prereqs
	fmt.Printf("Ensuring proxy service to istio-ingressgateway is in place ... ")
	if _, err = k8sServices.Get(proxyServiceName, metav1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			fail(err)
		}

		istioService, err := kube.CoreV1().Services(istioNamespace).Get(istioGatewayName, metav1.GetOptions{})
		if err != nil {
			fail(err)
		}

		servicePorts := istioService.Spec.Ports
		for i, port := range servicePorts {
			port.NodePort = 0
			servicePorts[i] = port
		}
		proxyService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      proxyServiceName,
			},
			Spec: corev1.ServiceSpec{
				Ports: servicePorts,
			},
		}

		_, err = k8sServices.Create(proxyService)
		if err != nil {
			fail(err)
		}

		endpointPorts := make([]corev1.EndpointPort, len(servicePorts))
		for i, port := range servicePorts {
			endpointPorts[i] = corev1.EndpointPort{
				Name:     port.Name,
				Port:     port.Port,
				Protocol: port.Protocol,
			}
		}
		proxyEndpoints := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      proxyServiceName,
			},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					IP: istioService.Spec.ClusterIP,
				}},
				Ports: endpointPorts,
			}},
		}
		_, err = endpoints.Create(proxyEndpoints)
		if err != nil {
			fail(err)
		}
		ok()
	} else {
		fmt.Println("EXISTED")
	}

	// Conversion
	fmt.Print("Converting deployment to Knative Service ... ")
	ksvc, err := conversion.ConvertToService(deployment)
	failIfError(err)

	fmt.Printf("Creating Knative Service '%s' ... ", ksvc.Name)
	_, err = knServices.Create(ksvc)
	failIfError(err)

	fmt.Printf("Waiting for Knative Service '%s' to become ready ... ", ksvc.Name)
	err = wait.NewWaitForReady("service", knServices.Watch, func(obj runtime.Object) (apis.Conditions, error) {
		service := obj.(*serving_v1alpha1_api.Service)
		return apis.Conditions(service.Status.Conditions), nil
	}).Wait(ksvc.Name, 10*time.Minute, ioutil.Discard)
	failIfError(err)

	// Cutting over
	fmt.Printf("Cutting over from deployment '%s' to Knative Service '%s' ...", deploymentName, ksvc.Name)
	for i := 0; i <= 10; i++ {
		weight := int32(i * 10)
		oldWeight := 100 - weight

		fmt.Printf("\rCutting over from deployment '%s' to Knative Service '%s' ... %d%% ... ", deploymentName, ksvc.Name, weight)

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
	fmt.Println("You can now clean up old resources safely.")
}

func failIfError(err error) {
	if err != nil {
		fail(err)
	}
	ok()
}

func fail(err error) {
	color.Red("FAILED: %v", err)
	os.Exit(1)
}

func ok() {
	color.Green("OK")
}

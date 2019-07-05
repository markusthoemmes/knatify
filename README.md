# :rocket: Knatify - Go serverless without even thinking about it!

[![asciicast](https://asciinema.org/a/7M1i9fLmB0fDCxgX3IvNxNdbE.svg)](https://asciinema.org/a/7M1i9fLmB0fDCxgX3IvNxNdbE)

**Knatify** is a tool (or set of tools) that make it dead simple to migrate existing Kubernetes deployments into Knative Services. On a higher level, this repository contains two different tools:

- **d2s**: `d2s` is a pipable program that transforms a given deployment YAML into a Knative Service.
- **knatify**: `knatify` itself is a higher-level tool that migrates existing deployments (existing as-in: already applied and running in a cluster) onto Knative Services. It also takes care of migrating the Openshift Route pointing to said deployment over to the new Knative Service in an incremental and safe way. Traffic to the Knative Service will be shifted over over time to ensure a seamless migration and no downtime for your services at all.

## Get it!

To install both tools via `go get`, run the following command:

```
$ go get github.com/markusthoemmes/knatify/cmd/knatify github.com/markusthoemmes/knatify/cmd/d2s
```

## Usage

### `knatify`

To use `knatify`, make sure `kubectl` or `oc` are correctly set up, pointing at the Openshift cluster of your choice. Watch the asciinema linked above to see it in action.

```console
$ knatify -namespace $NAMESPACE -deployment $DEPLOYMENTNAME -route $ROUTENAME
```

You can follow the interactive output of the program to see its progress. Once the program exists successfully, the route will point 100% to the new Knative Service so you can safely remove the old deployment and secondary resources like services and autoscalers.

**Note:** It is very important that the route's host property reflects exactly what Knative's computed host will look like. By default that will be `$SERVICE.$NS.$OPENSHIFT_HOST`. The behavior of Knative in that regard is customizable via the `config-domain` and `config-network` ConfigMaps in the `knative-serving` namespace.

---

### `d2s`

As mentioned, `d2s` is a simple pipable tool that works on given YAML.

```console
$ cat example.yaml | d2s
{"kind":"Service","apiVersion":"serving.knative.dev/v1alpha1","metadata":{"name":"frontend","creationTimestamp":null},"spec":{"template":{"metadata":{"creationTimestamp":null},"spec":{"containers":[{"name":"","image":"markusthoemmes/guestbook","ports":[{"containerPort":80}],"env":[{"name":"GET_HOSTS_FROM","value":"dns"}],"resources":{"requests":{"cpu":"100m","memory":"100Mi"}}}]}}},"status":{}}
```

The produced output can be piped into `kubectl` for example to create the Knative Service straight away.

```console
$ cat example.yaml | d2s | kubectl apply -f -
```
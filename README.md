# DN-VTAP

DN-VTAP is a Go Kubernetes operator that configures pod traffic mirroring through a `Kokotap` custom resource. It creates a sender pod running `/bin/kokotap_pod` alongside the target pod on the same node, with instructions to send mirrored traffic to a VXLAN receiver.

The project was scaffolded with Operator SDK for a master's thesis. This repository contains the Kubernetes API, controller, and deployment manifests. The Kokotap packet-mirroring binary, its container image build, and the receiver implementation are external to this repository.

## How it works

The controller watches namespaced `networking.dn-lab.io/v1alpha1` `Kokotap` resources. For each resource, it:

1. Looks for a sender pod named `kokotapped-<spec.podName>` in `spec.namespace`.
2. If no sender exists, reads the target pod's first container status, node name, and host IP.
3. Labels the target pod `dn-vtap=tapped` and creates the sender on that same node using `spec.nodeName` on the generated Pod.
4. Runs the sender with host networking, a privileged container, and host mounts for `/proc` and the container runtime socket.
5. Adds the finalizer `kokotap.networking.dn-lab.io` to the custom resource after creating the sender.

The intended traffic path is:

```text
Target pod interface (spec.podInterface)
    -> Kokotap sender on the same node
    -> VXLAN over UDP port 4789
    -> receiver at spec.destIp, using spec.vxlanID
```

The sender receives the target's full container ID, the node's host IP as `--vxlan-egressip`, and the requested mirror direction. The mirror interface name (`mirror`) and VXLAN port (`4789`) are hard-coded in the controller. DN-VTAP does not deploy or configure the receiver.

On deletion, the controller attempts to delete the sender, changes the target pod's label to `dn-vtap=not-tapped`, and removes the finalizer. See [Current limitations](#current-limitations) for cleanup edge cases.

## Prerequisites

- A Kubernetes cluster with Linux worker nodes and access through `kubectl`. Check the intended cluster with `kubectl config current-context`.
- A target pod that is already running, with a populated first container ID, assigned node, and host IP.
- **Docker or CRI-O on the target node.** The controller only configures mounts for these runtime prefixes; containerd is not supported by the current implementation.
- Admission policy in the target namespace that permits privileged pods, host networking, and hostPath mounts. The sender accesses `/proc` and either `/var/run/docker.sock` or `/var/run/crio/crio.sock` on the node.
- A reachable VXLAN receiver configured for the same VNI (`vxlanID`) and UDP port `4789`. The network path must permit that traffic and accommodate VXLAN encapsulation overhead.
- A sender image containing `/bin/kokotap_pod` and supporting the flags used by the controller. The repository samples reference `netand593/kokotap:2.0-beta`; verify its availability and suitability for your nodes.
- For the build workflow: Go, Make, Docker (or a compatible `CONTAINER_TOOL`), and a registry accessible to the cluster. The module declares Go 1.20 and the Dockerfile uses `golang:1.20`.

The manager runs as a non-root container, but the sender runs privileged. The default RBAC grants the manager access to pods and Kokotap resources across namespaces. A custom resource can target a namespace other than its own through `spec.namespace`, so access to creating Kokotap resources also grants access to request privileged tapping there.

## Deploy the controller

Build and push a controller image, then deploy it:

```sh
export IMG=your-registry/dn-vtap:your-tag
make docker-build docker-push IMG="$IMG"
make deploy IMG="$IMG"
kubectl rollout status deployment/dn-vtap-controller-manager -n dn-vtap-system
```

Replace the image value with a registry and tag you can push to. `make docker-build` runs `make test` before building. Make targets download helper tools into `bin/` as needed; tests also require envtest control-plane binaries.

`make deploy` installs the CRD, RBAC, and controller in `dn-vtap-system` using `config/default`. It also updates the image entry in `config/manager/kustomization.yaml`. Install the controller and CRD before creating a Kokotap resource.

### Run the controller locally

For development, install the CRD and run the controller against your kubeconfig context:

```sh
make install
make run
```

The controller runs in the foreground using your local Kubernetes credentials. Sender pods still run inside the cluster and require the same node runtime, privileges, and receiver setup. Use this as an alternative to the deployed controller; avoid running both against the same resources, since local leader election is disabled by default.

## Create a tap

Save this example as `kokotap.yaml`. Replace `my-app-pod` with an existing pod in `default`, set the receiver IP, and select a compatible sender image:

```yaml
apiVersion: networking.dn-lab.io/v1alpha1
kind: Kokotap
metadata:
  name: my-app-tap
  namespace: default
spec:
  podName: my-app-pod
  namespace: default
  destIp: "192.168.1.109"
  vxlanID: 1001
  mirrorType: both
  podInterface: eth0
  image: netand593/kokotap:2.0-beta
```

Confirm that the target is ready, then apply the resource:

```sh
kubectl wait --for=condition=Ready pod/my-app-pod -n default --timeout=120s
kubectl apply -f kokotap.yaml
kubectl get kokotaps -n default
kubectl get pod kokotapped-my-app-pod -n default -o wide
kubectl logs kokotapped-my-app-pod -n default -c kokotap-network-tap
```

The two namespace fields have different roles: `metadata.namespace` locates the Kokotap resource, while `spec.namespace` locates both the target and sender pods. Keeping them the same makes operations simpler.

**Do not apply the bundled samples unchanged.** They contain environment-specific pod names and addresses, and both quote `vxlanID` as a string. The CRD requires an integer, as shown above. Apply a specific edited manifest rather than `kubectl apply -f config/samples/`, which also includes a Kustomize configuration file.

### Custom resource fields

All seven fields below are required within `spec`; the CRD supplies no defaults.

| Field | Type | Meaning |
| --- | --- | --- |
| `podName` | string | Exact name of the existing pod to mirror; workload names and label selectors are not supported. |
| `namespace` | string | Namespace containing the target pod and the generated sender pod. |
| `destIp` | string | Receiver IP passed as `--vxlan-ip`. |
| `vxlanID` | integer (int32) | VXLAN VNI passed as `--vxlan-id`; use an unquoted YAML integer. |
| `mirrorType` | string | Intended values: `both`, `ingress`, or `egress`, passed as `--mirrortype`. |
| `podInterface` | string | Target pod interface passed as `--mirrorif`, for example `eth0`. |
| `image` | string | Sender image containing `/bin/kokotap_pod`; image pull policy is `Always`. This is separate from the controller image set by `IMG`. |

The generated CRD checks field types and presence within `spec`, but does not validate IP syntax, the VNI range, or the mirror direction. Although the Go API declares a `MirrorType` enum, the spec field uses a plain string, so that enum is not enforced. `status.conditions` exists in the API but is not populated by the controller.

## Observe and troubleshoot

Inspect the controller, sender, and Kubernetes events:

```sh
kubectl logs -n dn-vtap-system deployment/dn-vtap-controller-manager -c manager
kubectl describe kokotap my-app-tap -n default
kubectl describe pod kokotapped-my-app-pod -n default
kubectl logs kokotapped-my-app-pod -n default -c kokotap-network-tap
kubectl get events -n default --sort-by=.metadata.creationTimestamp
```

| Symptom | What to check |
| --- | --- |
| API rejects the manifest | Confirm `vxlanID` is an integer and all required spec fields are present. |
| No sender pod appears | Confirm the target pod exists in `spec.namespace`, has a container ID, and the controller can update pods and create privileged senders. |
| `Unsupported CRI` in controller logs | Check the target container ID prefix. Only `docker://` and `cri-o://` have runtime mount configuration. |
| Sender has image pull or startup errors | Check image accessibility, node architecture, `/bin/kokotap_pod`, and the host runtime socket path. |
| Sender runs but no traffic arrives | Check sender logs, target interface, mirror direction, receiver VNI, routing, UDP port 4789, and path MTU. A running sender alone does not prove successful mirroring. |
| Resource remains terminating | Check whether the sender or target disappeared before cleanup; see the deletion limitations below. |

The manager exposes `/healthz` and `/readyz` on port `8081` by default. These are process health checks, not traffic-mirroring checks. The deployment enables leader election and includes a metrics RBAC proxy on port `8443`. The `--metrics-bind-address` flag is parsed in `cmd/main.go` but is not passed into the manager options, so it currently does not change the metrics binding.

## Stop a tap and uninstall

Delete the Kokotap resource while the controller is still running:

```sh
kubectl delete -f kokotap.yaml
kubectl get pod kokotapped-my-app-pod -n default
kubectl get pod my-app-pod -n default --show-labels
```

After successful cleanup, the sender should be absent and the target label should read `dn-vtap=not-tapped`. Check for leftover sender pods before uninstalling, especially if a sender was not running at deletion time.

After deleting all taps and verifying cleanup:

```sh
make undeploy
```

The default deployment includes the CRD, so `make undeploy` removes it along with the controller and RBAC. For a local development setup where only `make install` was used, remove the CRD with `make uninstall`. Removing the CRD removes its custom resources across all namespaces.

## Current limitations

The controller implements creation and basic deletion, with the following constraints:

- **Existing senders are not updated.** If the expected sender pod exists, reconciliation returns without comparing its configuration. Delete the Kokotap resource and verify cleanup before recreating it with new settings. Editing `podName` or `namespace` can leave the previous sender behind.
- **Only Kokotap resources are watched.** There are no pod watches, owner references, or explicit periodic requeues. Sender deletion, container restarts, and workload replacement do not themselves trigger reconciliation or refresh the captured container ID.
- **One sender name per target pod.** Resources targeting the same pod share `kokotapped-<podName>` and can interfere with each other, even when the resources are in different namespaces. Use only one Kokotap resource per target.
- **Target lookup assumes a usable first container status.** Missing pods or empty container statuses can cause a panic; malformed or unsupported container IDs are logged without cleanly stopping creation. Create taps only after checking the target pod.
- **Deletion is incomplete for some pod states.** The controller only issues a sender deletion when its phase is `Running`. Other phases can leave an orphan sender after the finalizer is removed. If either pod is missing during cleanup, reconciliation returns an error and the resource can remain terminating.
- **Finalizer installation follows sender creation.** If that resource update fails, a subsequent reconciliation that finds the sender does not repair the missing finalizer.
- **Status does not report readiness or errors.** Use pod state, events, controller logs, and receiver-side observation to verify behavior.

## Development

| Path | Purpose |
| --- | --- |
| `api/v1alpha1/` | Kokotap API types, group registration, and generated deep-copy code. |
| `internal/controller/kokotap_controller.go` | Reconciliation, pod discovery, sender construction, and deletion. |
| `cmd/main.go` | Manager startup, flags, health checks, and controller registration. |
| `config/crd/` | Generated CRD and Kustomize configuration. |
| `config/rbac/`, `config/manager/`, `config/default/` | Permissions and deployment manifests. |
| `config/samples/` | Example resources requiring edits before use. |
| `internal/controller/suite_test.go` | Ginkgo/Gomega envtest setup and teardown. |
| `Dockerfile`, `Makefile` | Controller image, build, test, deployment, and operator bundle targets. |

Useful commands:

```sh
make help                 # List project targets
make generate manifests   # Regenerate deep-copy code, CRD, and RBAC after API changes
make fmt vet              # Format and statically check Go code
make build                # Build bin/manager (also generates, formats, and vets)
make test                 # Run tests using envtest
```

The module pins Kubernetes libraries to `v0.28.4` and controller-runtime to `v0.16.5`. The Makefile selects envtest Kubernetes assets `1.27.1`, controller-gen `v0.12.0`, and Kustomize `v5.0.1`; these are build/test dependencies, not a verified cluster compatibility matrix. The envtest helper installation uses `@latest`, so it is not pinned with the other tools.

The current test suite contains envtest setup and teardown but no reconciliation test cases. It does not validate runtime socket access, packet mirroring, or receiver behavior; those need a suitable cluster and receiver.

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


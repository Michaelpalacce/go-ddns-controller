# go-ddns-controller

![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)

A Kubernetes controller for managing DDNS records. 

## Description

This project is a Kubernetes controller that manages DDNS records. It's purpose is to be installed on a Kubernetes cluster
running on a network with a public IP address. The controller will monitor the public IP address of the cluster and update
the DNS records of the specified domain to point to the new IP address.

### What is DDNS?

Dynamic DNS (DDNS or DynDNS) is a method of automatically updating a name server in the Domain Name System (DNS), 
often in real time, with the active DDNS configuration of its configured hostnames, addresses or other information.

## Providers

Providers allow the controller to interact with different DNS providers. The controller will use the provider to update the DNS records of the specified domain.

Example Provider CRD:
```yaml
apiVersion: ddns.stefangenov.site/v1alpha1
kind: Provider
metadata:
  labels:
    app.kubernetes.io/name: go-ddns-controller
    app.kubernetes.io/managed-by: kustomize
  name: cloudflare-provider
spec:
  name: Cloudflare
  secretName: cloudflare
  configMap: cloudflare-config
  notifierRefs:
    - name: webhook-notifier
  customIPProvider: https://myIpProvider.example.com
```

Each provider has both a secret and a config map. The secret contains the credentials needed to authenticate with the provider's API.
The config map contains the configuration needed to interact with the provider. 
The provider also has a list of notifiers that will be triggered when the provider updates the DNS records. The notifierRefs are optional.

### Supported Providers

#### Cloudflare

The Cloudflare provider allows the controller to interact with the Cloudflare API to update the DNS records of the specified domain.

##### Secret

| Key | Description |
| --- | ----------- |
| apiToken | The Cloudflare API token |

##### Config Map

The configMap contains one key `config`.

The value of the `config` key is a JSON object with the following properties:
```json
{
  "cloudflare": {
      "zones": [
          {
              "name": "stefangenov.site",
              "records": [
                  {
                      "name": "stefangenov.site",
                      "proxied": true
                  }
              ]
          }
      ]
  }
}
```

## Notifiers

Notifiers allow the controller to send notifications when the DNS records are updated. 
The controller will trigger the notifiers when the provider updates the DNS records.

Example Notifier CRD:
```yaml
apiVersion: ddns.stefangenov.site/v1alpha1
kind: Notifier
metadata:
  labels:
    app.kubernetes.io/name: go-ddns-controller
    app.kubernetes.io/managed-by: kustomize
  name: webhook-notifier
spec:
  name: Webhook
  secretName: webhook
  configMap: webhook-config
```

Each notifier has both a secret and a config map. The secret contains the credentials needed to authenticate with the notifier's API.
The config map contains the configuration needed to interact with the notifier.

### Supported Notifiers

#### Webhook

The Webhook notifier allows the controller to send a POST request to a specified URL when the DNS records are updated.

##### Secret

| Key | Description |
| --- | ----------- |
| URL | The URL to send the message to. |

##### Config Map

The configMap contains one key `config`. The value of `config` is "" for now.

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=ghcr.io/michaelpalacce/go-ddns-controller:latest
```

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=ghcr.io/michaelpalacce/go-ddns-controller:latest
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

### Building the project

To build the project, run the following command:

```sh
make release-prepare
```

This command will generate the all needed manifests and prepare parts of the helm chart.

### Running the tests

To run the tests, run the following command:

```sh
make coverage
```

This command will run the tests and generate a coverage report. The coverage report will automatically be visualized in the browser.

## Project Distribution

The helm chart is located in the `chart` directory. The chart is used to deploy the controller to a Kubernetes cluster.
Take a look at the `values.yaml` file to see the default values for the chart.

```sh
helm upgrade --install go-ddns-controller charts/go-ddns-controller \
    --create-namespace \
    --namespace go-ddns-controller-system \
    --set image.repository=${REPOSITORY} \
    --set image.tag=${VERSION} \
    --set controller.replicas=${REPLICAS}
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

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


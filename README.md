# Conduit Operator

This reposistory contains a kubernetes operator for deploying Conduit instances.
The operator extends the Kubernetes API with a custom `Conduit` resource.

The operator is based on [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

## Overview

Conduit pipelines are represented as Conduit custom resources, where each pipeline will be provisioned 
as a distinct Conduit instance with its own lifecycle. 

The Conduit custom resource definition format is very similar to that of [pipeline configurations](https://conduit.io/docs/pipeline-configuration-files/getting-started).

```yaml
apiVersion: operator.conduit.io/v1alpha
kind: Conduit
metadata:
  name: conduit-generator
spec:
  running: true
  name: generator.log
  description: generator pipeline
  connectors:
    - name: source-connector
      type: source
      plugin: builtin:generator
      settings:
        - name: format.type
          value: structured
        - name: format.options.id
          value: "int"
        - name: format.options.name
          value: "string"
        - name: format.options.company
          value: "string"
        - name: format.options.trial
          value: "bool"
        - name: recordCount
          value: "3"
    - name: destination-connector
      type: destination
      plugin: builtin:log
```

The operator can install standalone connectors referred by their github org / repo names.
Version can optionally be specified or the latest will be used. Format is `github-org/repo-name`, e.g. `conduitio/conduit-connector-generator`

```yaml
apiVersion: operator.conduit.io/v1alpha
kind: Conduit
metadata:
  name: conduit-generator
spec:
  running: true
  name: generator.log
  description: generator pipeline
  connectors:
    - name: source-connector
      type: source
      plugin: conduitio/conduit-connector-generator
      settings:
        - name: format.type
          value: structured
        - name: format.options.id
          value: "int"
        - name: format.options.name
          value: "string"
        - name: format.options.company
          value: "string"
        - name: format.options.trial
          value: "bool"
        - name: recordCount
          value: "3"
    - name: destination-connector
      type: destination
      plugin: conduitio/conduit-connector-log
```
### Schema registry

As of [v0.11.0](https://conduit.io/changelog/2024-08-19-conduit-0-11-0-release) Conduit supports the use of a schema registry. 
This allows connectors to automatically extract or use the schema referred to by the OpenCDC record to encode/decode data. 

By default conduit uses a builtin schema registry, however in certain use cases a schema registry needs to be shared between
multiple instances. The conduit resource allows for schema registry to be defined as of [v0.2.0](https://github.com/ConduitIO/conduit-operator/releases/tag/v0.0.2).

```yaml
apiVersion: operator.conduit.io/v1alpha
kind: Conduit
metadata:
  name: conduit-generator-schema-registry
spec:
  running: true
  name: generator.standalone.log
  description: generator pipeline
  schemaRegistry:
    url: http://apicurio:8080/apis/ccompat/v7
    # basicAuthUser:
    #   - value: <schemaUser>
    # basicAuthPassword:
    #   - secretRef:
    #     key: schema-registry-password
    #     name: schema-registry-secret
  connectors:
    - name: source-connector
      type: source
      plugin: conduitio/conduit-connector-generator
      settings:
        - name: format.type
          value: structured
        - name: format.options.id
          value: "int"
        - name: format.options.name
          value: "string"
        - name: format.options.company
          value: "string"
        - name: format.options.trial
          value: "bool"
        - name: recordCount
          value: "3"
    - name: destination-connector
      type: destination
      plugin: conduitio/conduit-connector-log
      pluginVersion: v0.4.0
```


## Quickstart

1. Download this repository locally
   
2. Run `make dev` to initialize a [kind cluster](https://kind.sigs.k8s.io) and install the operator using helm.
   
3. Deploy a sample pipeline configuration
   ```shell
   kubectl apply -f config/samples/conduit-generator.yaml
   ```

4. Wait for instance to become ready
   ```
   kubectl wait --for=condition=Ready -l app.kubernetes.io/name=conduit-server-conduit-generator pod
   ```

5. Follow logs of provisioned instalce
   ```
   kubectl logs -f -l app.kubernetes.io/name=conduit-server-conduit-generator
   ```
## Deployment

The operator provides a [helm chart](charts/conduit-operator) which can be used for deployment on any cluster.

Additional metadata can be injected in each provisioned conduit instance via `controller.conduitMetadata` configuration. 

For example, to instruct a prometheus instance to scrape each Conduit instance metrics:

```yaml
controller:
  conduitMetadata:
    podAnnotations:
      prometheus.io/scrape: true
      prometheus.io/path: /metrics
      prometheus.io/port: 8080
```

For more configuration options see [charts/conduit-operator/values.yaml](charts/conduit-operator/values.yaml)

### Using the helm chart repository

Alternatively the operator can be deployed via the helm repository
To add the repository to your helm repos:

```shell
helm repo add conduit https://conduitio.github.io/conduit-operator
```

Install the operator in the `conduit-operator` namespace of your cluster:

```shell
helm install conduit-operator conduit/conduit-operator --create-namespace -n conduit-operator
```



## Development

Changes to the operator can be deployed to the kind cluster via `make dev` 

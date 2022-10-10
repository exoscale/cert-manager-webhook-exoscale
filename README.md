# cert-manager Webhook for Exoscale

cert-manager Webhook for Exoscale is [cert-manager-webhook](https://cert-manager.io/docs/configuration/acme/dns01/webhook/) allowing users to use [Exoscale DNS](https://community.exoscale.com/documentation/dns/) for DNS01 challenge.
Based on [Example Webhook](https://github.com/cert-manager/webhook-example).

## Getting started

### Prerequisites

- [Exoscale API access key](https://community.exoscale.com/documentation/iam/quick-start/)
- A valid domain configured on [Exoscale](https://community.exoscale.com/documentation/dns/)
- A Kubernetes cluster
- [Helm](https://helm.sh/) [installed](https://helm.sh/docs/intro/install/)
- [cert-manager](https://cert-manager.io/docs/installation/)

> Note: if you want to restrict Exoscale API access key make sure the following operations are allowed: `list-dns-domains`, `list-dns-domain-records`, `get-dns-domain-record`, `get-operation`, `create-dns-domain-record` and `delete-dns-domain-record`.

### Installing

Once everything is set up, install Exoscale Webhook:
```bash
git clone https://github.com/exoscale/cert-manager-webhook-exoscale.git
cd cert-manager-webhook-exoscale
helm install exoscale-webhook ./deploy/exoscale-webhook
```

### How to use it

> Note: official cert-manager documentation is available [here](https://cert-manager.io/docs/usage/).

In the following examples we will create a new certificate for domain `example.com`.
Both cert-manager and cert-manager-webhook-exoscale should be already running in the cluster as described in the previous chapter.

First step is to create a secret containing the Exoscale API credentials. Create the `secret.yaml` file with the following content:

> Note: static credentials can be configured as environment variables but it is not recommended for production use.

```yaml
apiVersion: v1
stringData:
  EXOSCALE_API_KEY: <YOUR-EXOSCALE-API-KEY>
  EXOSCALE_API_SECRET: <YOUR-EXOSCAL-API-SECRET>
kind: Secret
metadata:
  name: exoscale-secret
type: Opaque
```

And run:
```bash
kubectl create -f secret.yaml
```

To create a cert-manager `Issuer`, create a  `issuer.yaml` file with the following content:

> Note: following example uses staging letsencrypt server and dummy email address, make sure to update those for production use.

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: exoscale-issuer
spec:
  acme:
    email: my-user@example.com
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: exoscale-private-key-secret
    solvers:
    - dns01:
        webhook:
          groupName: acme.exoscale.com
          solverName: exoscale
          config:
            apiKeyRef:
              key: EXOSCALE_API_KEY
              name: exoscale-secret
            apiSecretRefRef:
              key: EXOSCALE_API_SECRET
              name: exoscale-secret
```

Then run:
```bash
kubectl create -f issuer.yaml
```

Now create the `certificate.yaml` file with the following content:
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-com
spec:
  dnsNames:
  - example.com
  issuerRef:
    name: exosacle-issuer
  secretName: example-com-tls
```

And run:
```bash
kubectl create -f certificate.yaml
```

After a bit certificate should be in the ready state:
```bash
$ kubectl get certificate example-com
NAME          READY   SECRET            AGE
example-com   True    example-com-tls   0m52s
```

### Debugging

To see more detailed logs, set one or both environment variables to any value:
- `EXOSCALE_DEBUG`: shows debug logs;
- `EXOSCALE_API_TRACE`: prints API requests/responses.

Easiest way to set them is through helm (exposed as `env.debug` and `env.trace`):

```
helm install exoscale-webhook ./deploy/exoscale-webhook --set env.debug=1 --set env.trace=1
```

## Integration testing

Before running the test, you need:
- A valid domain managed by Exoscale (examples here use `example.com`)
- The variables `EXOSCALE_API_KEY` and `EXOSCALE_API_SECRET` in the environment

In order to run the integration tests, run:
```bash
TEST_ZONE_NAME=example.com make test
```


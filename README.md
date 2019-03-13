# Config Controller for Alertmanager

This Config Controller is based on the [Grafana Operator](https://github.com/tsloughter/grafana-operator) project. The Config Controller should be run within [Kubernetes](https://github.com/kubernetes/kubernetes) as a sidecar with the [Prometheus Alertmanager](https://github.com/prometheus/alertmanager).

It watches for new/updated/deleted *ConfigMaps* and if they define the specified annotations as `true` it will save each resource from ConfigMap to Alertmanagers local storage and reload the Alertmanager. This requires Alertmanager 0.16.x.

## ConfigMap Annotations


Currently it supports three resources:

**1. Receiver**

`alertmanager.net/receiver` with values: `"true"` or `"false"`

**2. Route**

`alertmanager.net/route` with values: `"true"` or `"false"`

**3. Inhibit Rule**

`alertmanager.net/inhibit_rule` with values `"true"` or `"false"`

**Config**

`alertmanager.net/config` with values: `"true"` or `"false"`

`alertmanager.net/key` with values: `string`

Alertmanager will start with a provided minimal dummy config, which is definetly valid. Then the Config Controller will load the *ConfigMap* with annotation `alertmanager.net/config: true`, which includes the global Alertmanager configuration. The Config Controller will merge this configuration with the dummy configuration and only if this configuration is valid, Alertmanager will be reloaded. For each Alertmanager Setup there should be only one *ConfigMap* with annotation `alertmanager.net/config: true`. If you want to run e.g. three Alertmanagers in HA mode (replicas = 3), then all three Alertmanager will load the same ConfigMap and have the exact same config. To prevent other "nonadmin" users from misusing of `alertmanager.net/config`, a `key` is used. If and only if the `key` in *ConfigMap* matches the `key` in args of the Alertmanager Controller, the Controller will use the *ConfigMap*.

**Id**

`alertmanager.net/id` with values: `"0"` ... `"n"`

In case of multiple Alertmanager *setups* in same Kubernetes Cluster all the ConfigMaps have to be mapped to the right Alertmanager setup.
So each *ConfigMap* can be additionaly annotated with the `alertmanager.net/id` (if not, the default `id` will be `"0"`)

You can run e.g. three Alertmanagers in HA mode with id=0 and for an another setup with three Alertmanagers in HA mode with id=1, and so on.

**Note**

Mentioned `"true"` values can be also specified with: `"1", "t", "T", "true", "TRUE", "True"`

Mentioned `"false"` values can be also specified with: `"0", "f", "F", "false", "FALSE", "False"`

ConfigMap examples can be found [here](configmap-examples).

## Usage
```
--run-outside-cluster # Uses local ~/.kube/config rather than in cluster configuration
--reloadUrl # Sets the URL to reload Alertmanager
--configPath # Sets the path to use to store config files
--configTemplate # Sets the location of template of the Alertmanager config
--id # Sets the ID, so the Controller knows which ConfigMaps should be watched
--key # Sets the key, so the Controller can recognize the template of config in ConfigMap
```

## Development
### Dependencies
[Glide](https://glide.sh/) is a package management tool for Go. To install dependencies:
```console
$ glide update
$ glide install
```

### Build
```
go build -v -i -o ./bin/alertmanager-config-controller ./cmd # on Linux
GOOS=linux CGO_ENABLED=0 go build -v -i -o ./bin/alertmanager-config-controller ./cmd # on macOS/Windows
```
To build a docker image out of it, look at provided [Dockerfile](Dockerfile) example.


## Deployment
Our preferred way to install alertmanager/alertmanager-config-controller is [Helm](https://helm.sh/). See example installation at our [Helm directory](helm) within this repo.

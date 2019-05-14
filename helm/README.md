### Installing the Chart

To install the chart with the release name `alertmanager` in namespace `monitoring`:

```console
$ helm upgrade alertmanager charts/alertmanager --namespace monitoring --install
```
The command deploys alertmanager with alertmanager-controller on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

> **Tip**: List all releases using `helm list`

### Uninstalling the Chart

To uninstall/delete the `alertmanager` deployment:

```console
$ helm delete alertmanager --purge
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration
The following table lists the configurable parameters of the alertmanager chart and their default values.

Parameter | Description | Default
--------- | ----------- | -------
`replicaCount` | The number of pod replicas | `1`
`init.repository` | init container image repository | `busybox`
`init.tag` | init container image tag | `"1.30"`
`init.resources.limits.cpu` | init container resources limits for cpu | `10m`
`init.resources.limits.memory` | init container resources limits for memory | `10Mi`
`init.resources.requests.cpu` | init container resources limits for cpu | `1m`
`init.resources.requests.memory` | init container resources limits for memory | `5Mi`
`alertmanager.image.repository` | alertmanager container image repository | `prom/alertmanager`
`alertmanager.image.tag` | alertmanager container image tag | `v0.17.0`
`alertmanager.configFile` | Alertmanager config file | `/etc/alertmanager/alertmanager.yml`
`alertmanager.storagePath` | Alertmanager storage path | `/alertmanager`
`alertmanager.meshListenAddress` | Alertmanager mesh listen address | `6783`
`alertmanager.resources.limits.cpu` | alertmanager container resources limits for cpu | `20m`
`alertmanager.resources.limits.memory` | alertmanager container resources limits for memory | `64Mi`
`alertmanager.resources.requests.cpu` | alertmanager container resources limits for cpu | `10m`
`alertmanager.resources.requests.memory` | alertmanager container resources limits for memory | `32Mi`
`alertmanagerConfigController.image.repository` | alertmanager-config-controller container image repository | `dockerregistry/alertmanager-config-controller`
`alertmanagerConfigController.image.tag` | alertmanager-config-controller container image tag | `0.1.0`
`alertmanagerConfigController.url` | The url to reload alertmanager | `http://alertmanager:9093/-/reload`
`alertmanagerConfigController.id` | The id to specify alertmanager | `0`
`alertmanagerConfigController.key` | The key to specify alertmanager config template | `q5!sder6P`
`alertmanagerConfigController.configPath` | The path to use to store config files | `/etc/config`
`alertmanagerConfigController.configTemplate` | The location of template of the Alertmanager config | `/etc/alertmanager/alertmanager.tmpl`
`alertmanagerConfigController.logLevel` | The log-level of alertmanager-config-controller | `info`
`service.port` | The port the alertmanager uses | `9093`
`ingress.host` | The host of ingress url | `alertmanager.xxx.yyy`

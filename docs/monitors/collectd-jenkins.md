<!--- GENERATED BY gomplate from scripts/docs/monitor-page.md.tmpl --->

# collectd/jenkins

 Monitors jenkins by using the
[jenkins collectd Python
plugin](https://github.com/signalfx/collectd-jenkins), which collects
metrics from jenkins instances

Sample YAML configuration:

```yaml
monitors:
- type: collectd/jenkins
  host: 127.0.0.1
  port: 8080
  metricsKey: reallylongmetricskey
```

Sample YAML configuration with specific enhanced metrics included

```yaml
monitors:
- type: collectd/jenkins
  host: 127.0.0.1
  port: 8080
  metricsKey: reallylongmetricskey
  includeMetrics:
  - "vm.daemon.count"
  - "vm.terminated.count"
```

Sample YAML configuration with all enhanced metrics included

```yaml
monitors:
- type: collectd/jenkins
  host: 127.0.0.1
  port: 8080
  metricsKey: reallylongmetricskey
  enhancedMetrics: true
```


Monitor Type: `collectd/jenkins`

[Monitor Source Code](https://github.com/signalfx/signalfx-agent/tree/master/internal/monitors/collectd/jenkins)

**Accepts Endpoints**: **Yes**

**Multiple Instances Allowed**: Yes

## Configuration

| Config option | Required | Type | Description |
| --- | --- | --- | --- |
| `host` | **yes** | `string` |  |
| `port` | **yes** | `integer` |  |
| `metricsKey` | **yes** | `string` | Key required for collecting metrics.  The access key located at `Manage Jenkins > Configure System > Metrics > ADD.` If empty, click `Generate`. |
| `enhancedMetrics` | no | `bool` | Whether to enable enhanced metrics |
| `includeMetrics` | no | `list of string` | Used to enable individual enhanced metrics when `enhancedMetrics` is false |
| `username` | no | `string` | User with security access to jenkins |
| `apiToken` | no | `string` | API Token of the user |
| `sslKeyFile` | no | `string` | Path to the keyfile |
| `sslCertificate` | no | `string` | Path to the certificate |
| `sslCACerts` | no | `string` | Path to the ca file |




## Metrics

The following table lists the metrics available for this monitor. Metrics that are not marked as Custom are standard metrics and are monitored by default.

| Name | Type | Custom | Description |
| ---  | ---  | ---    | ---         |
| `gauge.jenkins.job.duration` | gauge |  | Time taken to complete the job in ms. |
| `gauge.jenkins.node.executor.count.value` | gauge |  | Total Number of executors in an instance |
| `gauge.jenkins.node.executor.in-use.value` | gauge |  | Total number of executors being used in an instance |
| `gauge.jenkins.node.health-check.score` | gauge |  | Mean health score of an instance |
| `gauge.jenkins.node.health.disk.space` | gauge |  | Binary value of disk space health |
| `gauge.jenkins.node.health.plugins` | gauge |  | Boolean value indicating state of plugins |
| `gauge.jenkins.node.health.temporary.space` | gauge |  | Binary value of temporary space health |
| `gauge.jenkins.node.health.thread-deadlock` | gauge |  | Boolean value indicating a deadlock |
| `gauge.jenkins.node.online.status` | gauge |  | Boolean value of instance is reachable or not |
| `gauge.jenkins.node.queue.size.value` | gauge |  | Total number pending jobs in queue |
| `gauge.jenkins.node.slave.online.status` | gauge |  | Boolean value for slave is reachable or not |
| `gauge.jenkins.node.vm.memory.heap.usage` | gauge |  | Percent utilization of the heap memory |
| `gauge.jenkins.node.vm.memory.non-heap.used` | gauge |  | Total amount of non-heap memory used |
| `gauge.jenkins.node.vm.memory.total.used` | gauge |  | Total Memory used by instance |


To specify custom metrics you want to monitor, add a `metricsToInclude` filter
to the agent configuration, as shown in the code snippet below. The snippet
lists all available custom metrics. You can copy and paste the snippet into
your configuration file, then delete any custom metrics that you do not want
sent.

Note that some of the custom metrics require you to set a flag as well as add
them to the list. Check the monitor configuration file to see if a flag is
required for gathering additional metrics.

```yaml

metricsToInclude:
  - metricNames:
    monitorType: collectd/jenkins
```





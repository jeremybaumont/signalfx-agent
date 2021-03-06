<!--- GENERATED BY gomplate from scripts/docs/monitor-page.md.tmpl --->

# collectd/genericjmx

 Monitors Java services that expose metrics on
JMX using collectd's GenericJMX plugin.

See https://github.com/signalfx/integrations/tree/master/collectd-genericjmx
and https://collectd.org/documentation/manpages/collectd-java.5.shtml

Example (gets the thread count from a standard JMX MBean available on all
Java JMX-enabled apps):

```yaml

monitors:
 - type: collectd/genericjmx
   host: my-java-app
   port: 7099
   mBeanDefinitions:
     threading:
       objectName: java.lang:type=Threading
       values:
       - type: gauge
         table: false
         instancePrefix: jvm.threads.count
         attribute: ThreadCount
```
## Troubleshooting

Exposing JMX in your Java application can be a tricky process.  Oracle has a
[helpful guide for Java
8](https://docs.oracle.com/javase/8/docs/technotes/guides/management/agent.html)
that explains how to expose JMX metrics automatically by setting Java
properties on your application.  Here are a set of Java properties that are
known to work with Java 7+:

```
java \
  -Dcom.sun.management.jmxremote.port=5000 \
  -Dcom.sun.management.jmxremote.authenticate=false \
  -Dcom.sun.management.jmxremote.ssl=false \
  -Dcom.sun.management.jmxremote.rmi.port=5000 \
  ...
```

This should work as long as the agent is allowed to access port 5000 on the
Java app's host (i.e. there is no firewall blocking it).  Note that this
does not enable authentication or encryption, but these can be added if
desired.

Assuming you have the `host` config set to `172.17.0.3` and the port set to
`5000` (this is a totally arbitrary port and your JMX app will probably be
something different), here are some errors you might receive and their
meanings:

### Connection Refused
```
Creating MBean server connection failed: java.io.IOException: Failed to retrieve RMIServer stub: javax.naming.ServiceUnavailableException [Root exception is java.rmi.ConnectException: Connection refused to host: 172.17.0.3; nested exception is:
     java.net.ConnectException: Connection refused (Connection refused)]
```

This error indicates that the JMX connect port is not open on the specified
host.  Confirm (via netstat/ss or some other tool) that this port
is indeed open on the configured host, and is listening on an appropriate
address (i.e. if the agent is running on a remote server then JMX should not
be listening on localhost only).

### RMI Connection Issues

```
Creating MBean server connection failed: java.rmi.ConnectException: Connection refused to host: 172.17.0.3; nested exception is:
     java.net.ConnectException: Connection timed out (Connection timed out)
```

This indicates that the JMX connect port was reached successfully, but the
RMI port that it was directed to is being blocked, probably by a firewall.
The easiest thing to do here is to make sure the
`com.sun.management.jmxremote.rmi.port` property in your Java app is set to
the same port as the JMX connect port.  There may be other variations of
this that say `Connection reset` or `Connection refused` but they all
generaly indicate a similar cause.

## Useful links

 - https://realjenius.com/2012/11/21/java7-jmx-tunneling-freedom/


Monitor Type: `collectd/genericjmx`

[Monitor Source Code](https://github.com/signalfx/signalfx-agent/tree/master/internal/monitors/collectd/genericjmx)

**Accepts Endpoints**: **Yes**

**Multiple Instances Allowed**: Yes

## Configuration

| Config option | Required | Type | Description |
| --- | --- | --- | --- |
| `host` | **yes** | `string` | Host to connect to -- JMX must be configured for remote access and accessible from the agent |
| `port` | **yes** | `integer` | JMX connection port (NOT the RMI port) on the application.  This correponds to the `com.sun.management.jmxremote.port` Java property that should be set on the JVM when running the application. |
| `name` | no | `string` |  |
| `serviceName` | no | `string` | This is how the service type is identified in the SignalFx UI so that you can get built-in content for it.  For custom JMX integrations, it can be set to whatever you like and metrics will get the special property `sf_hostHasService` set to this value. |
| `serviceURL` | no | `string` | The JMX connection string.  This is rendered as a Go template and has access to the other values in this config. NOTE: under normal circumstances it is not advised to set this string directly - setting the host and port as specified above is preferred. (**default:** `service:jmx:rmi:///jndi/rmi://{{.Host}}:{{.Port}}/jmxrmi`) |
| `instancePrefix` | no | `string` |  |
| `username` | no | `string` |  |
| `password` | no | `string` |  |
| `customDimensions` | no | `map of string` | Takes in key-values pairs of custom dimensions at the connection level. |
| `mBeansToCollect` | no | `list of string` | A list of the MBeans defined in `mBeanDefinitions` to actually collect. If not provided, then all defined MBeans will be collected. |
| `mBeansToOmit` | no | `list of string` | A list of the MBeans to omit. This will come handy in cases where only a few MBeans need to omitted from the default list |
| `mBeanDefinitions` | no | `map of object (see below)` | Specifies how to map JMX MBean values to metrics.  If using a specific service monitor such as cassandra, kafka, or activemq, they come pre-loaded with a set of mappings, and any that you add in this option will be merged with those.  See [collectd GenericJMX](https://collectd.org/documentation/manpages/collectd-java.5.shtml#genericjmx_plugin) for more details. |


The **nested** `mBeanDefinitions` config object has the following fields:

| Config option | Required | Type | Description |
| --- | --- | --- | --- |
| `objectName` | no | `string` |  |
| `instancePrefix` | no | `string` |  |
| `instanceFrom` | no | `list of string` |  |
| `values` | no | `list of object (see below)` |  |
| `dimensions` | no | `list of string` |  |


The **nested** `values` config object has the following fields:

| Config option | Required | Type | Description |
| --- | --- | --- | --- |
| `type` | no | `string` |  |
| `table` | no | `bool` |  (**default:** `false`) |
| `instancePrefix` | no | `string` |  |
| `instanceFrom` | no | `list of string` |  |
| `attribute` | no | `string` |  |









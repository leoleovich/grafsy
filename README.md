# Description

This is a very light proxy for graphite metrics with additional features:
- Taking metrics from network (see [configuration](https://github.com/leoleovich/grafsy#configuration)) or from file directly
- Buffering metrics if Graphite itself is down
- Function of summing/averaging metrics with a special prefix (see [configuration](https://github.com/leoleovich/grafsy#configuration))
- Filtering 'bad' metrics, which are not passing check against regexp
- Periodical sending to Graphite server to avoid traffic pikes

# Configuration

There is a config file which must be located under */etc/grafsy/grafsy.toml*
Most of the time you need to use default (recommended) configuration of grafsy, but you can always modify params:

## Base

- clientSendInterval - the interval, after which client will send data to graphite. In seconds
- maxMetrics - Maximum amount of metrics, which will be processed in one ClientSendTimeout
    In case of problems with connection/amount of metrics, this configuration will take up to maxMetrics\*clientSendInterval\*50(AVG size of metric) = 5MB of data on disk.
    Also these 2 params are exactly allocating memory.
- allowedMetrics - Regexp of allowed metric. Every metric which is not passing check against regexp will be removed
- log - Main log file

## Sending and cache

- graphiteAddr - Real Graphite server to which client will send all data
- localBind - Local address:port for local daemon
- metricDir - Directory, in which developers/admins... can write any file with metrics
- retryFile - Data, which was not sent will be buffered in this file

## Sum

- sumPrefix - Prefix for metric to sum. Do not forget to include it in allowedMetrics if you change it
- sumInterval - Summing up interval for metrics with prefix "sumPrefix". In seconds
- sumsPerSecond - Amount of sums which grafsy performs per second. If grafsy receives more metrics than sumsPerSecond*sumInterval - rest will be dropped

## AVG

- avgPrefix - Prefix for metric to calculate average. Do not forget to include it in allowedMetrics if you change it
- avgInterval - Summing up interval for metrics with prefix "sumPrefix". In seconds
- avgPerSecond - Amount of avg which grafsy performs per second. If grafsy receives more metrics than avgsPerSecond*avgInterval - rest will be dropped

## Monitoring

- grafsyPrefix - Prefix to send statistic from grafsy itself. Set null to not send monitoring. E.g **grafsyPrefix**.testserver.grafsy.{sent,dropped,got...}
- grafsySuffix - Suffix to send statistic from grafsy itself. Set null to not send monitoring. E.g testserver.**grafsySuffix**.grafsy.{sent,dropped,got...}

# Installation

- Install go https://golang.org/doc/install
- Make a proper structure of directories: ```mkdir -p /opt/go/src /opt/go/bin /opt/go/pkg```
- Setup g GOPATH variable: ```export GOPATH=/opt/go```
- Clone this project to src: ```go get github.com/leoleovich/grafsy```
- Fetch dependencies: ```cd /opt/go/github.com/leoleovich/grafsy && go get ./...```
- Compile project: ```go install github.com/leoleovich/grafsy```
- Copy config file: ```mkdir /etc/grafsy && /opt/go/src/github.com/leoleovich/grafsy/grafsy.toml /etc/grafsy/```
- Change your settings, e.g. ```graphiteAddr```
- Create a log folder: ```mkdir -p /var/log/grafsy``` or run grafsy for user, which has permissions to create logfiledir
- Run it ```/opt/go/bin/grafsy```

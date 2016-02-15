# Description

This is a very light proxy for graphite metrics with additional features:
- Taking metrics from network (see configuration) or from file directly
- Buffering metrics if graphite itself is down
- Function of summing metrics with special prefix (see configuration)
- Filtering 'bad' metrics, which are not passing check against regexp
- Periodical sending to graphite server to avoid traffic pikes

# Configuration

There is a config file which must be located under /etc/grafsy/grafsy.toml
Most of the time you need to use default (recommended) configuration of grafsy, but you can always modify params:
- clientSendInterval - the interval, after which client will send data to graphite. In seconds
- maxMetrics - Maximum amount of metrics, which will be processed in one ClientSendTimeout. In case of problems with connection/amount of metrics, this configuration will take up to maxMetrics\*clientSendInterval\*50 = 5MB of data on disk. Also these 2 params are exactly allocating memory
- graphiteAddr - Real Graphite server to which client will send all data
- localBind - Local address:port for local daemon
- log - Main log file
- metricDir - Directory, in which developers/admins... can write any file with metrics
- retryFile - Data, which was not sent will be buffered in this file
- sumPrefix - Prefix for metric to sum
- sumInterval - Summing up interval for metrics with prefix "sumPrefix". In seconds
- grafsyPrefix - Prefix to send statistic from grafsy itself. Set null to not send monitoring. E.g **grafsyPrefix**.testserver.grafsy.{sent,dropped,got...}
- grafsySuffix - Suffix to send statistic from grafsy itself. Set null to not send monitoring. E.g testserver.**grafsySuffix**.grafsy.{sent,dropped,got...}
- allowedMetrics - Regexp of allowed metric. Every metric which is not passing check against regexp will be removed
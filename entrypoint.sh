#!/usr/bin/env bash

set -e

clientSendInterval=${clientSendInterval:-10}
metricsPerSecond=${metricsPerSecond:-1000}
# this is the only one ENV that does not have defaults
carbonAddrs=${carbonAddrs}
connectTimeout=${connectTimeout:-2}
localBind=${localBind:-"localhost:3002"}

log=${log:-"-"}

metricDir=${metricDir:-"/tmp/grafsy/metrics"}
useACL=${useACL:-false}

retryDir=${retryDir:-"/tmp/grafsy/retry"}

sumPrefix=${sumPrefix:-"SUM."}
avgPrefix=${avgPrefix:-"AVG."}
minPrefix=${minPrefix:-"MIN."}
maxPrefix=${maxPrefix:-"MAX."}
aggrInterval=${aggrInterval:-60}
aggrPerSecond=${aggrPerSecond:-100}

monitoringPath=${monitoringPath:-"servers.HOSTNAME.software"}

allowedMetrics=${allowedMetrics:-'^((SUM|AVG|MIN|MAX)[.])?[-a-zA-Z0-9_]+[.][-a-zA-Z0-9_().:/,{}=+#]+(\\s)[-0-9[.]eE+]+(\\s)[0-9]{10}$'}


default_run() {
  if [ -z "${carbonAddrs}" ]; then
    echo "carbonAddrs must be set as space separated addresses"
    exit 1
  fi
  if ! [ -e /etc/grafsy/grafsy.toml ]; then
    cat > /etc/grafsy/grafsy.toml << EOF
clientSendInterval = $clientSendInterval

metricsPerSecond = $metricsPerSecond

carbonAddrs = [
    $(sed 's/^/"/;s/$/"/;s/\s\+/", "/g' <<< "$carbonAddrs")
]
connectTimeout = $connectTimeout

localBind = "$localBind"

log = "$log"

metricDir = "$metricDir"
useACL = $useACL

retryDir = "$retryDir"

sumPrefix = "$sumPrefix"
avgPrefix = "$avgPrefix"
minPrefix = "$minPrefix"
maxPrefix = "$maxPrefix"
aggrInterval = $aggrInterval
aggrPerSecond = $aggrPerSecond

monitoringPath = "$monitoringPath"

allowedMetrics = "$allowedMetrics"
EOF
  fi
  exec "$@"
}

if [ "$*" = "/grafsy/grafsy" ]; then
  default_run "$@"
else
  exec "$@"
fi

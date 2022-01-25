set -ex
if [ -f /splunk-messages/environ ]; then
  . /splunk-messages/environ
fi
/otelcol $@

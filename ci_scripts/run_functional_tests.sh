#!/usr/bin/env bash
set -e
pyenv global 3.9.1
pip install --upgrade pip
pip install -r test/requirements.txt
cd test
#Run pytests
echo "Running functional tests....."
python -m pytest \
	--splunkd-url https://$CI_SPLUNK_HOST:8089 \
	--splunk-user admin \
	--splunk-password $CI_SPLUNK_PASSWORD \
	-p no:warnings -s

#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
APP_NAME="cloud-connector"  # name of app-sre "application" folder this component lives in
COMPONENT_NAME="cloud-connector-api"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
IMAGE="quay.io/cloudservices/cloud-connector"

IQE_PLUGINS="rhc"
IQE_MARKER_EXPRESSION=""
IQE_FILTER_EXPRESSION="cloud_connector"
IQE_CJI_TIMEOUT="10m"


# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh > .cicd_bootstrap.sh && source .cicd_bootstrap.sh

source $CICD_ROOT/build.sh
source $CICD_ROOT/deploy_ephemeral_env.sh
source $CICD_ROOT/cji_smoke_test.sh

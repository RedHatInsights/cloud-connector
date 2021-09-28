#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
APP_NAME="cloud-connector"  # name of app-sre "application" folder this component lives in
COMPONENT_NAME="cloud-connector"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
IMAGE="quay.io/cloudservices/cloud-connector"

IQE_PLUGINS="iqe-rhc-plugin"
IQE_MARKER_EXPRESSION="smoke"
IQE_FILTER_EXPRESSION="cloud-connector"
IQE_CJI_TIMEOUT="10m"


# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh -o bootstrap.sh
source bootstrap.sh  # checks out bonfire and changes to "cicd" dir...

source $CICD_ROOT/build.sh
source $APP_ROOT/unit_test.sh
source $CICD_ROOT/deploy_ephemeral_env.sh
#source $CICD_ROOT/cji_smoke_test.sh

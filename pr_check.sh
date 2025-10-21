#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
APP_NAME="cloud-connector"  # name of app-sre "application" folder this component lives in
COMPONENT_NAME="cloud-connector"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
IMAGE="quay.io/cloudservices/cloud-connector"
IQE_CJI_TIMEOUT="30m"
EXTRA_DEPLOY_ARGS="-p playbook-dispatcher/CLOUD_CONNECTOR_IMPL=impl"
IQE_MARKER_EXPRESSION="not e2e"
IQE_ENV_VARS="DYNACONF_USER_PROVIDER__rbac_enabled=false"


AVAILABLE_CLUSTERS=('ephemeral' 'crcd')
# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh > .cicd_bootstrap.sh && source .cicd_bootstrap.sh

source $CICD_ROOT/build.sh
source $CICD_ROOT/deploy_ephemeral_env.sh

# Run Cloud Connector isolated tests
COMPONENT_NAME="cloud-connector"
IQE_PLUGINS="cloud-connector"
source $CICD_ROOT/cji_smoke_test.sh

# Run RHC Contract integration tests
COMPONENT_NAME="playbook-dispatcher"
IQE_PLUGINS="rhc-contract"
IQE_IMAGE_TAG="rhc-contract"
source $CICD_ROOT/cji_smoke_test.sh
source $CICD_ROOT/post_test_results.sh

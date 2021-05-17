#!/bin/bash

SERVICE_NAME=tagent
CURRENT_VERSION=v3.6.0
BACKUP_PATH=${BACKUP_PATH:-"/tmp/"}
INSTALLED_EXEC_PATH="/opt/trustagent/bin/$SERVICE_NAME"
CONFIG_PATH="/opt/trustagent/configuration/"
NEW_EXEC_NAME="$SERVICE_NAME"
LOG_FILE=${LOG_FILE:-"/tmp/$SERVICE_NAME-upgrade.log"}
echo "" > $LOG_FILE
./upgrade.sh -s $SERVICE_NAME -v $CURRENT_VERSION -e $INSTALLED_EXEC_PATH -c $CONFIG_PATH -n $NEW_EXEC_NAME -b $BACKUP_PATH |& tee -a $LOG_FILE

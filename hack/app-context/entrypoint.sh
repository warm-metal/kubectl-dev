#!/usr/bin/env bash

[[ -z "${APP_ROOT}" ]] && echo "set env APP_ROOT" && exit 2

mkdir -p ${APP_ROOT}/proc
mount -t proc /proc ${APP_ROOT}/proc

mkdir -p ${APP_ROOT}/dev
mount -o rbind /dev ${APP_ROOT}/dev

mkdir -p ${APP_ROOT}/sys
mount -o rbind /sys ${APP_ROOT}/sys

mkdir -p ${APP_ROOT}/run
mount -o rbind /run ${APP_ROOT}/run

mkdir -p ${APP_ROOT}/root
mount -o rbind /root ${APP_ROOT}/root

tail -f /dev/null
#!/usr/bin/env bash

exporter() {
   /shell_exporter --collector.textfile.directory ${DIR} --web.listen-address ${PORT} 
}

crond() {
    /usr/sbin/cron &
}

main() {
    crond
    exporter
}

main

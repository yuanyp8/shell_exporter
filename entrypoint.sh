#!/usr/bin/env bash

exporter() {
   /shell_exporter --collector.textfile.directory ${DIR} --web.listen-address ${PORT} &
}

crond() {
    /usr/sbin/crond -n 
}


add_cron() {
mkdir -p /var/spool/cron/
cat >> /var/spool/cron/root <<EOF
*/1 * * * * /cron.sh
EOF
chmod +x /cron.sh
}

main() {
    exporter
    add_cron
    crond
    /bin/bash
}

main

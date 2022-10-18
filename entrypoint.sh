#!/usr/bin/env bash

exporter() {
   /shell_exporter --collector.textfile.directory ${DIR} --web.listen-address ${PORT} &
}

crond() {
    /usr/sbin/cron &  
}


add_cron() {
#mkdir -p /var/spool/cron/
#cat >> /var/spool/cron/root <<EOF
#*/1 * * * * /cron.sh
#EOF
task='*/1 * * * * /cron.sh'
echo "$(crontab -l)" | grep "^${task}$" &>/dev/null || echo -e "$(crontab -l)\n${task}" | crontab -
chmod +x /cron.sh
}

main() {
    exporter
    add_cron
    crond
    /bin/bash
}

main

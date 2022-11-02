#!/bin/bash

#set -x

# *.prom 
monitor_file=/exporters/test.prom   

URL="https://devops.onecode.cmict.cloud"

#ret=curl https://devops.onecode.cmict.cloud -k | grep login | wc -l

echo "# HELP online_serve Tell determine if the given url is available." > $monitor_file
echo "# TYPE online_serve counter" >> $monitor_file
echo 'online_serve{app=\"kubesphere\"}' `curl https://devops.onecode.cmict.cloud -k | grep login | wc -l`>> $monitor_file


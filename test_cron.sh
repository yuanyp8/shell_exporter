#!/bin/bash

#set -x

monitor_file=/exporters/test.prom

echo "# HELP test_metrics  Tell the summury info." > $monitor_file 
echo "# TYPE test_metrics  counter" >> $monitor_file
echo "test_metrics{metrics_name=\"test\"}" 1  >> $monitor_file


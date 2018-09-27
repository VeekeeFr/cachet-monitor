#!/bin/bash

export SCRIPT=$(readlink -f $0)
export SCRIPTPATH=`dirname $SCRIPT`

if [ -f /www/scripts/hook-prestart.sh ]
then
	echo "Using prestart hook"
	/www/scripts/hook-prestart.sh
fi

echo "Starting cachet process"
/usr/local/bin/cachet_monitor -c /www/conf/cachet-monitor.yml --log=/www/log/cachet-monitor.log

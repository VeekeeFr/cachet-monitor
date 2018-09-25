#!/usr/bin/env bash

export SCRIPT=$(readlink -f $0)
export SCRIPTPATH=`dirname $SCRIPT`

if [ -f /www/scripts/hook-prestart.sh ]
then
	/www/scripts/hook-prestart.sh
fi

/usr/local/bin/cachet_monitor -c /www/conf/cachet-monitor.yml --log=/www/log/cachet-monitor.log

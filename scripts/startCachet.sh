#!/bin/bash

export SCRIPT=$(readlink -f $0)
export SCRIPTPATH=`dirname $SCRIPT`

if [ -f /www/scripts/hook-prestart.sh ]
then
	echo "Using prestart hook"
	/www/scripts/hook-prestart.sh
fi

echo "Starting cachet process"
/usr/local/bin/cachet_monitor --config=/www/conf/cachet-monitor.yml --log=/www/log/cachet-monitor.log
RETURN_CODE=$?

echo "Return code: ${RETURN_CODE}"
if [ ${RETURN_CODE} -gt 0 ]
then
	echo "Configuration data..."
	cat /www/conf/cachet-monitor.yml

	echo "##################################"

	echo "Latest log entries..."
	tail -20 /www/log/cachet-monitor.log
fi
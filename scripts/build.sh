#!/usr/bin/env bash

export SCRIPT=$(readlink -f $0)
export SCRIPTPATH=`dirname $SCRIPT`

function usage
{
	echo "Usage:"
	echo "       $0 -build"
	echo "       $0 -download"
	echo "       $0 -run <config file>"
}

while [ "$1" != "" ]; do
    case $1 in
	-h | --h | -help | --help)
		usage
		exit 0
		;;
	-b | --b | -build | --build)
		echo "Building..."
		cd ${SCRIPTPATH}/..
		go build -ldflags "-X main.AppBranch=local -X main.Build=unknown -X main.BuildDate=`date +%Y-%m-%d_%H:%M:%S`" -o cachet_monitor
		if [ $? -gt 0 ]
		then
			echo "Error detected..."
			exit 1
		fi
		;;
	-d | --d | -download | --download)
		echo "Downloading..."
		cd ${SCRIPTPATH}/..
		wget -O cachet_monitor https://github.com/VeekeeFr/cachet-monitor/releases/download/snapshot/cachet_monitor
		if [ $? -gt 0 ]
		then
			echo "Error detected..."
			exit 1
		fi
		;;
	-r | --r | -run | --run)
		shift
		echo "Running using '${1}'"
		cd ${SCRIPTPATH}/..
		CACHET_DEV="true"
		./cachet_monitor -c ${1}
		;;
	* )
   		echo "ERROR: Argument '$1' is not supported"
		usage
                exit 1
    esac
    shift
done



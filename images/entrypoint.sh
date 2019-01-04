#!/bin/bash

# Always exit on errors.
set -e

# Set known directories.
CNI_BIN_DIR="/host/opt/cni/bin"
SRIOV_BIN_FILE="/usr/bin/sriov"

# Give help text for parameters.
function usage()
{
    echo -e "This is an entrypoint script for SR-IOV CNI to overlay its"
    echo -e "binary into location in a filesystem. The binary file will"
    echo -e "be copied to the corresponding directory."
    echo -e ""
    echo -e "./entrypoint.sh"
    echo -e "\t-h --help"
    echo -e "\t--cni-bin-dir=$CNI_BIN_DIR"
    echo -e "\t--sriov-bin-file=$SRIOV_BIN_FILE"
}

# Parse parameters given as arguments to this script.
while [ "$1" != "" ]; do
    PARAM=`echo $1 | awk -F= '{print $1}'`
    VALUE=`echo $1 | awk -F= '{print $2}'`
    case $PARAM in
        -h | --help)
            usage
            exit
            ;;
        --cni-bin-dir)
            CNI_BIN_DIR=$VALUE
            ;;
        --sriov-bin-file)
            SRIOV_BIN_FILE=$VALUE
            ;;
        *)
            echo "ERROR: unknown parameter \"$PARAM\""
            usage
            exit 1
            ;;
    esac
    shift
done


# Create array of known locations
declare -a arr=($CNI_BIN_DIR $SRIOV_BIN_FILE)

# Loop through and verify each location each.
for i in "${arr[@]}"
do
  if [ ! -e "$i" ]; then
    echo "Location $i does not exist"
    exit 1;
  fi
done

# Copy file into proper place.
cp -f $SRIOV_BIN_FILE $CNI_BIN_DIR

echo "Entering sleep... (success)"

# Sleep forever.
sleep infinity

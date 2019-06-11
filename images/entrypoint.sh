#!/bin/sh

# Always exit on errors.
set -e

# Set known directories.
CNI_BIN_DIR="/host/opt/cni/bin"
SRIOV_BIN_FILE="/usr/bin/sriov"

# Give help text for parameters.
usage()
{
    /bin/echo -e "This is an entrypoint script for SR-IOV CNI to overlay its"
    /bin/echo -e "binary into location in a filesystem. The binary file will"
    /bin/echo -e "be copied to the corresponding directory."
    /bin/echo -e ""
    /bin/echo -e "./entrypoint.sh"
    /bin/echo -e "\t-h --help"
    /bin/echo -e "\t--cni-bin-dir=$CNI_BIN_DIR"
    /bin/echo -e "\t--sriov-bin-file=$SRIOV_BIN_FILE"
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
            /bin/echo "ERROR: unknown parameter \"$PARAM\""
            usage
            exit 1
            ;;
    esac
    shift
done


# Loop through and verify each location each.
for i in $CNI_BIN_DIR $SRIOV_BIN_FILE
do
  if [ ! -e "$i" ]; then
    /bin/echo "Location $i does not exist"
    exit 1;
  fi
done

# Copy file into proper place.
cp -f $SRIOV_BIN_FILE $CNI_BIN_DIR

echo "Entering sleep... (success)"

# Sleep forever. 
# sleep infinity is not available in alpine; instead lets go sleep for ~68 years. Hopefully that's enough sleep
sleep 2147483647

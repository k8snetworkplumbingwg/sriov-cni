#!/bin/sh

# Always exit on errors.
set -e

# Set known directories.
CNI_BIN_DIR="/host/opt/cni/bin"
SRIOV_BIN_FILE="/usr/bin/sriov"

# Give help text for parameters.
usage()
{
    printf "This is an entrypoint script for SR-IOV CNI to overlay its\n"
    printf "binary into location in a filesystem. The binary file will\n"
    printf "be copied to the corresponding directory.\n"
    printf "\n"
    printf "./entrypoint.sh\n"
    printf "\t-h --help\n"
    printf "\t--cni-bin-dir=%s\n" $CNI_BIN_DIR
    printf "\t--sriov-bin-file=%s\n" $SRIOV_BIN_FILE
}

# Parse parameters given as arguments to this script.
while [ "$1" != "" ]; do
    PARAM=$(echo "$1" | awk -F= '{print $1}')
    VALUE=$(echo "$1" | awk -F= '{print $2}')
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
cp -f "$SRIOV_BIN_FILE" "$CNI_BIN_DIR"

echo "Entering sleep... (success)"
trap : TERM INT

# Sleep forever. 
# sleep infinity is not available in alpine; instead lets go sleep for ~68 years. Hopefully that's enough sleep
sleep 2147483647 & wait

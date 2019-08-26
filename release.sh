#!/bin/bash -e

MIN_GBP='0.9.12'
GBP_VERSION="$(gbp --version)"
if ! (echo "gbp ${MIN_GBP}"; echo "${GBP_VERSION}") | sort -Vc 2>/dev/null ; then
  echo "Minimal version for git-buildpackage is ${MIN_GBP}. Provided is ${GBP_VERSION}"
  exit 1
fi

#===  FUNCTION  ================================================================
#         NAME:  usage
#  DESCRIPTION:  Display usage information.
#===============================================================================
function usage ()
{
	echo "Usage :  $0 [options] [--]

    Options:
    -h|help        Display this message
    -n|new-version New version for changelog"

}    # ----------  end of function usage  ----------

#-----------------------------------------------------------------------
#  Handle command line arguments
#-----------------------------------------------------------------------

while getopts ":hn:" opt
do
  case $opt in

	h )  usage; exit 0   ;;

	n ) NEW_VERSION="${OPTARG}" ;;

	: ) echo "Invalid option: -$OPTARG requires an argument" 1>&2; exit 1 ;;

	* )  echo -e "\n  Option does not exist : $OPTARG\n"
		  usage; exit 1   ;;

  esac    # --- end of case ---
done
shift $((OPTIND -1))

if [ -z "${NEW_VERSION}" ]; then
  echo 'New debian version is not set' 1>&2
  exit 1
fi

gbp dch --git-author --distribution=stretch --force-distribution --ignore-branch --commit -N "${NEW_VERSION}" --auto
git tag "v${NEW_VERSION}"

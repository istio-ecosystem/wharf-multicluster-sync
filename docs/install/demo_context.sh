#!/bin/bash

# This script exports things, `source` it instead of executing it.

#MK#if [ -z "$3" ]
if [ -z "$2" ]
  then
    echo "Syntax: source demo_context.sh <ca-ctx> <cluster1-ctx> <cluster2-ctx> [<cluster3-ctx>]"
    echo "where ca-ctx, cluster1-ctx and cluster2-ctx are the Kubeconfig context names for the relevent clusters."
    return	# This script is sourced, not executed, so don't exit
fi
 
export CLUSTER1_NAME=$2
export CLUSTER2_NAME=$3
#MK#export CLUSTER3_NAME=$4
export CLUSTER1=$2
export CLUSTER2=$3
#MK#export CLUSTER3=$4
export ROOTCA_NAME=$1
export ROOTCA=$1

echo Checking access to ROOTCA cluster with context $ROOTCA_NAME
if ! kubectl --context $ROOTCA_NAME version --short=true ; then
   echo $ROOTCA_NAME not accessable
   return	# This script is sourced, not executed, so don't exit
fi

echo Checking access to Cluster 1 with context $CLUSTER1
if ! kubectl --context $CLUSTER1 version --short=true ; then
   echo $CLUSTER1 not accessable
   return	# This script is sourced, not executed, so don't exit
fi

echo Checking access to Cluster 2 with context $CLUSTER2
if ! kubectl --context $CLUSTER2 version --short=true ; then
   echo $CLUSTER2 not accessable
   return	# This script is sourced, not executed, so don't exit
fi

#MK#if ! [ -z "$CLUSTER3" ]
#MK#  then
#MK#	echo Checking access to Cluster 3 with context $CLUSTER3
#MK#	if ! kubectl --context $CLUSTER3 version --short=true ; then
#MK#	   echo $CLUSTER3 not accessable
#MK#	   return	# This script is sourced, not executed, so don't exit
#MK#	fi
#MK#fi

echo Success setting environment for clusters

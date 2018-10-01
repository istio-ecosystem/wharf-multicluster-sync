#!/bin/bash

if [ -z "$3" ]
  then
    echo "Syntax: source demo_context.sh ca-name cluster1-name cluster2-name"
    return
fi
 
export MB="kubernetes-admin@kubernetes"
export CLUSTER1_NAME=$2
export CLUSTER2_NAME=$3
export CLUSTER1=$2
export CLUSTER2=$3
export ROOTCA_NAME=$1

echo Checking ROOTCA $ROOTCA_NAME
if ! kubectl --context $ROOTCA_NAME version --short=true ; then
   echo $ROOTCA_NAME not accessable
   return
fi

echo Checking CLUSTER1 $CLUSTER1
if ! kubectl --context $CLUSTER1 version --short=true ; then
   echo $CLUSTER1 not accessable
   return
fi

echo Checking CLUSTER2 $CLUSTER2
if ! kubectl --context $CLUSTER2 version --short=true ; then
   echo $CLUSTER2 not accessable
   return
fi

echo Success setting environment for three clusters

#!/bin/bash
set +e

source ~/Sandbox/kube_context.sh


./delete_demo_3.sh
./delete_demo_2.sh
./delete_demo_1.sh

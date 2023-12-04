# simple implementation of raft consensus algorithm

This repository is a fork of the following repository:
https://github.com/chapin666/simple-raft \
It contains slight modifications to make the project easily runnable, but functionality should be pretty much identical

## Setup

The project can either be run through `goreman start` or with multiple terminal windows \
If you wish to run it with goreman please install goreman first with the following command

    go install github.com/mattn/goreman@latest

## Running the code
To start a node the following command should be run

    go run raft.go --id x --cluster IP_ADDRESS:PORT,IP_ADDRESS:PORT... --port :PORT

The paramaters are better described here:

* id: unique integer identifier for the node
* cluster: set of ip adresses and ports for all the **other** nodes in the cluster, comma seperated
* port: the port to be used for this specific process

An example execution of three nodes can easily be run in one terminal, if goreman is installed, with:

    goreman start

The code can also be run by manually starting nodes in different terminal windows. If there is any additional confusion regarding how to start an individual node, please look at the example execution in the **Procfile** file
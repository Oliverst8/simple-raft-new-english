# simple implementation of raft consensus algorithm

This repository is a fork of the following repository:
https://github.com/chapin666/simple-raft \
It contains slight modifications to make the project easily runnable, but functionality should be pretty much identical

## Setup

This project should be run with `goreman` \
If do not have goreman installed already please install it with:

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

The configuration of the startup can be modified by changing the **Procfile** file. For example, more nodes can be added to the cluster, or ports can be changed.
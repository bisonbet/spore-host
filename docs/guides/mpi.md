# MPI Clusters

For workloads that need to communicate across multiple nodes — large-scale simulations, distributed training, or parallel data processing — spore.host can launch a multi-node MPI cluster as a single command.

## Launch a cluster

```sh
spawn launch \
  --name climate-sim \
  --instance-type hpc7g.16xlarge \
  --count 8 \
  --mpi \
  --ttl 12h \
  --command "mpirun -n 128 ./simulate --config config.yaml"
```

`--count 8` launches 8 instances. `--mpi` sets up passwordless SSH between all nodes, installs OpenMPI if not present, and configures the hostfile. The `--command` runs on node 0 after all nodes are ready.

## EFA for high-performance networking

For workloads that saturate standard networking, enable Elastic Fabric Adapter:

```sh
spawn launch \
  --name distributed-training \
  --instance-type p4d.24xlarge \
  --count 4 \
  --mpi \
  --efa \
  --ttl 24h \
  --command "mpirun -n 32 --mca btl ^tcp python train.py"
```

`--efa` requires an EFA-supported instance type (p4d, hpc7g, c5n, and others). spore.host automatically selects an EFA-enabled security group and places all instances in a cluster placement group for lowest latency.

## Instance placement

By default, MPI clusters are placed in a cluster placement group for minimum latency. If you have an existing placement group:

```sh
spawn launch \
  --name sim \
  --count 16 \
  --mpi \
  --placement-group my-hpc-group
```

## Monitoring the cluster

```sh
spawn list --job-array climate-sim     # all nodes in the cluster
spawn status climate-sim-0             # status of the head node
spawn status climate-sim-1             # status of worker node 1
```

## Lifecycle

The cluster's TTL applies to all nodes. If the job completes before the TTL (using `--on-complete terminate`), all nodes terminate together. If a worker node fails, the head node gets a notification.

```sh
spawn launch \
  --name sim \
  --count 8 \
  --mpi \
  --ttl 12h \
  --on-complete terminate \
  --completion-file /tmp/SPAWN_COMPLETE \
  --command "mpirun -n 64 ./sim && touch /tmp/SPAWN_COMPLETE"
```

::: tip
Write the completion file only from the head node (rank 0). MPI programs run on all nodes, so guard the `touch` with `if [ $OMPI_COMM_WORLD_RANK -eq 0 ]; then touch /tmp/SPAWN_COMPLETE; fi`.
:::

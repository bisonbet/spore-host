# Parameter Sweeps

A parameter sweep runs the same job across many combinations of input parameters, each on its own instance, in parallel. It's useful for hyperparameter search, sensitivity analysis, and any scenario where you want to explore a parameter space without waiting for jobs to run sequentially.

## The basic pattern

```sh
spawn launch hp-search \
  --instance-type g5.xlarge \
  --ttl 4h \
  --params "learning_rate=0.001,0.01,0.1;batch_size=32,64,128" \
  --command "python train.py --lr {learning_rate} --batch {batch_size}"
```

This launches 9 instances (3 learning rates × 3 batch sizes), each running the training script with a different combination. Each instance has its own TTL and terminates independently when done.

## Parameter file format

For larger sweeps, define parameters in a YAML file:

```yaml
# sweep.yaml
defaults:
  instance_type: g5.xlarge
  ttl: 4h
  on_complete: terminate

params:
  - learning_rate: 0.001
    batch_size: 32
  - learning_rate: 0.001
    batch_size: 64
  - learning_rate: 0.01
    batch_size: 32
  # ... more combinations
```

```sh
spawn launch hp-search --param-file sweep.yaml
```

## Generating combinations automatically

Instead of listing every combination, use ranges and spore.host expands them:

```sh
spawn launch grid-search \
  --instance-type g5.xlarge \
  --ttl 2h \
  --params "learning_rate=log:0.0001:0.1:5;dropout=0.1,0.2,0.3,0.5" \
  --cartesian \
  --command "python train.py --lr {learning_rate} --dropout {dropout}"
```

`log:0.0001:0.1:5` generates 5 values logarithmically spaced between 0.0001 and 0.1.

## Monitoring a sweep

```sh
spawn list --sweep-name hp-search     # all instances in the sweep
spawn sweep status <sweep-id>         # summary: running, completed, failed
spawn sweep cancel <sweep-id>         # terminate all remaining instances
```

With Slack connected, you'll get a DM when the sweep finishes (all instances have terminated).

## Collecting results

Each instance writes its results to a path you control — typically S3. The convention is to include the sweep index or parameters in the path:

```sh
spawn launch hp-search \
  --instance-type g5.xlarge \
  --param-file sweep.yaml \
  --command "python train.py --lr {learning_rate} && \
             aws s3 cp results.json s3://my-bucket/sweeps/hp-search/{index}/results.json && \
             touch /tmp/SPAWN_COMPLETE" \
  --on-complete terminate
```

Each instance has `{index}` (0-based position in the sweep) and all parameter values available as environment variables and template substitutions.

## Cost estimation

Before launching a large sweep, use `--estimate-only` to see the maximum possible cost without launching anything:

```sh
spawn launch hp-search \
  --instance-type g5.xlarge \
  --param-file sweep.yaml \
  --ttl 4h \
  --estimate-only
```

This shows the maximum cost if every instance runs for the full TTL. Actual cost is lower because most instances complete before the TTL.

## Next steps

- [Job Arrays](/guides/job-arrays) — for when you want a fixed count of identical instances rather than parameterised jobs
- [Pipelines](/guides/pipelines) — chain sweeps so stage 2 launches after stage 1 completes

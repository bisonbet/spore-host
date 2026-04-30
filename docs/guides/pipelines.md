# Pipelines

A pipeline chains stages together: when one stage completes, it automatically launches the next. This is useful for multi-step workflows — data preparation, training, evaluation, post-processing — where each stage has different compute requirements.

## The basic pattern

Each stage uses `--on-complete` to trigger the next stage. The completion file (`/tmp/SPAWN_COMPLETE`) is the handoff signal.

```sh
# Stage 1: data preparation (cheap CPU instance)
spawn launch \
  --name pipeline-prep \
  --instance-type c6i.4xlarge \
  --ttl 4h \
  --on-complete terminate \
  --completion-file /tmp/SPAWN_COMPLETE \
  --command "python prepare_data.py --output s3://my-bucket/prepared/ && touch /tmp/SPAWN_COMPLETE"
```

Stage 1 terminates when it writes the completion file. Stage 2 runs on whatever compute you want:

```sh
# Stage 2: training (GPU instance)
spawn launch \
  --name pipeline-train \
  --instance-type p4d.24xlarge \
  --ttl 24h \
  --on-complete terminate \
  --command "python train.py --data s3://my-bucket/prepared/ && touch /tmp/SPAWN_COMPLETE"
```

## Automated pipeline definition

Instead of launching stages manually, define the full pipeline in a YAML file and let spore.host manage the handoffs:

```yaml
# pipeline.yaml
name: ml-pipeline

stages:
  - name: prep
    instance_type: c6i.4xlarge
    ttl: 4h
    command: python prepare_data.py --output s3://my-bucket/prepared/

  - name: train
    instance_type: p4d.24xlarge
    ttl: 24h
    spot: true
    command: python train.py --data s3://my-bucket/prepared/

  - name: eval
    instance_type: c6i.2xlarge
    ttl: 2h
    command: python evaluate.py --model s3://my-bucket/model/
```

```sh
spawn pipeline --file pipeline.yaml --slack-workspace T03NE3GTY
```

Each stage launches automatically when the previous one completes. You'll get a Slack notification at each stage transition.

## Passing data between stages

Stages communicate through shared storage — S3 is the most common approach, but any storage accessible from EC2 works (EFS, FSx for Lustre, etc.):

```sh
# Mount shared EFS across all stages
spawn pipeline \
  --file pipeline.yaml \
  --efs-id fs-0abc123 \
  --efs-mount /shared
```

All pipeline instances mount the EFS filesystem, so output from stage 1 at `/shared/prepared/` is immediately available to stage 2.

## Error handling

If a stage fails (exits with non-zero), the pipeline stops and you receive a notification. Check the stage log from Slack: `/spore status pipeline-train`

To resume from a failed stage:

```sh
spawn pipeline resume --file pipeline.yaml --from-stage train
```

## Monitoring

```sh
spawn pipeline status ml-pipeline    # current stage, elapsed time, cost
spawn pipeline cancel ml-pipeline    # stop current stage and all remaining
```

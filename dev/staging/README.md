This cloudbuild job is invoked by prow after every commit to master,
and will push images to the staging image repository.

It can also be run against your own GCP project, which is helpful when
testing or adding to the cloudbuild job.  An example command is:

```
# Choose a bucket to upload to
ARTIFACT_LOCATION=gs://$(gcloud config get-value project)-staging/etcdadm

gcloud builds submit --config=dev/staging/cloudbuild.yaml . \
    --substitutions=_DOCKER_IMAGE_PREFIX=$(gcloud config get-value project)/ \
    --substitutions=_ARTIFACT_LOCATION=${ARTIFACT_LOCATION}

gsutil ls -r ${ARTIFACT_LOCATION}/
```


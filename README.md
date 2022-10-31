# TotalRecall

Easy containerized PostgreSQL backups to S3.

## Getting Started

### Docker

A pre-built Docker image is available for immediate use. It's based on [the
official Docker PostgreSQL image](https://hub.docker.com/_/postgres), supporting
all documented environment variables. The image is [publicly available
here](https://hub.docker.com/repository/docker/furiousluke/totalrecall).

The TotalRecall binary and PostgreSQL background worker are added to the
standard PostgreSQL image, adding approximately 9mb to the original size.

TotalRecall requires some additional values to be set in order to operate:

 * `TOTALRECALL_DATABASE_ID`, a unique identifier required to distinguish your
   database.
 * `TOTALRECALL_ENCRYPTION_KEY`, a string used to encrypt all data sent to S3.
 * `TOTALRECALL_BUCKET`, the name of the S3 bucket to store backups.
 * `AWS_ACCESS_KEY_ID`, this key must have access to the S3 bucket in which
   backups will be stored.
 * `AWS_SECRET_ACCESS_KEY`.
 * `AWS_REGION`.

In addition to the options that can be supplied to the PostgreSQL docker image,
the following optional settings may be set for TotalRecall:

 * `TOTALRECALL_STORAGE_PREFIX`, an arbitrary prefix in which to store backups.
 
An example:
 
```bash
docker run \
  -e POSTGRES_PASSWORD=my-secret-password \
  -e TOTALRECALL_DATABASE_ID=38dc0ad2-af58-436a-a323-15f651ce0e54 \
  -e TOTALRECALL_ENCRYPTION_KEY=6F93DD1BB3EBF3A461C96FF5E91AA \
  -e TOTALRECALL_BUCKET=my-bucket \
  -e TOTALRECALL_STORAGE_PREFIX=db-backups \
  -e AWS_ACCESS_KEY_ID=myawsaccesskey \
  -e AWS_SECRET_ACCESS_KEY=mysecretaccesskey \
  -e AWS_REGION=ap-southeast-2 \
  furiousluke/totalrecall
```

Specifying secret keys directly on the command line is not generally
recommended. Typically, secrets will make their way into containers as files
located somewhere like `/run/secrets`. These may be specified as follows:

```bash
docker run \
  -e POSTGRES_PASSWORD_FILE=/run/secrets/database_password \
  -e TOTALRECALL_DATABASE_ID=file:/run/secrets/database_id \
  -e TOTALRECALL_ENCRYPTION_KEY=file:/run/secrets/backup_encryption_key \
  -e TOTALRECALL_BUCKET=my-bucket \
  -e TOTALRECALL_STORAGE_PREFIX=db-backups \
  -e AWS_ACCESS_KEY_ID=file:/run/secrets/aws_access_key_id \
  -e AWS_SECRET_ACCESS_KEY=file:/run/secrets/aws_secret_access_key \
  -e AWS_REGION=ap-southeast-2 \
  furiousluke/totalrecall
```

Please note the use of `POSTGRES_PASSWORD_FILE` instead of `POSTGRES_PASSWORD`.

### Building the image

From within the root directory of the cloned repository, build the Docker image
locally with:

```bash
docker build -t totalrecall ./postgres .
```

### Stand-alone

To build the TotalRecall binary:

```bash
make
```

To build the PostgreSQL worker process:

```bash
make worker
```

To build both the binary and the worker:

```bash
make
```

## Backup schedule and retention

Currently, TotalRecall is hard-coded to take a backup every 7 days. Of course,
this is not terribly useful for a variety of use-cases. There are a couple of
issues currently in the backlog to make the backup schedule and retention policy
configurable.

## Metrics

Metrics about the operations performed by TotalRecall are automatically stored
in a `totalrecall` schema within the database. A table named `metrics` will be
created, and each archive put/get command, backup, and restore, will be entered
into a row within said table.

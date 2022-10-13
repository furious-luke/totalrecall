#include <stdio.h>
#include <stdlib.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/file.h>
#include <unistd.h>
#include <signal.h>
#include <fcntl.h>
#include <errno.h>
#include "postgres.h"

#include "miscadmin.h"
#include "postmaster/bgworker.h"
#include "postmaster/interrupt.h"
#include "storage/ipc.h"
#include "storage/latch.h"
#include "storage/lwlock.h"
#include "storage/proc.h"
#include "storage/shmem.h"

#include "access/xact.h"
#include "executor/spi.h"
#include "fmgr.h"
#include "lib/stringinfo.h"
#include "pgstat.h"
#include "utils/builtins.h"
#include "utils/snapmgr.h"
#include "tcop/utility.h"

#include "worker.h"

PG_MODULE_MAGIC;

// extern void WorkerMain();

PGDLLEXPORT void background_main(Datum) pg_attribute_noreturn();

static pid_t child_pid = 0;
// static char* dbname = NULL;
static const char* schema_name = "totalrecall";
static const char* table_name = "metrics";

int
exec_backup(void) {
  elog(LOG, "TotalRecall: forking backup child process");
  child_pid = fork();
  if (child_pid == -1) {
    child_pid = 0;
    elog(LOG, "TotalRecall: failed to fork backup child process");
    return 1;
  }
  if (child_pid == 0) {
    if (execlp("totalrecall", "totalrecall", "backup", "--scheduled", (char*)NULL) == -1) {
      // TODO: Should log this somewhere.
      exit(1);
    }
  }
  for (;;) {
    pid_t done_pid;
    wait_latch(2 * 1000);
    CHECK_FOR_INTERRUPTS();  // TODO: Need to kill child process if we're interrupted.
    done_pid = waitpid(child_pid, NULL, WNOHANG);
    if (done_pid == -1) {
      elog(LOG, "TotalRecall: failed to check status of backup child process");
      // TODO: What to do here?
      exit(1);
    }
    if (done_pid == child_pid) {
      elog(LOG, "TotalRecall: backup process completed");
      return 0;
    }
  }
}

void
ingest_metrics(StringInfoData* buf) {
  FILE *fp;
  int lock;
  char line[256], filename[256];
  int n_ingested = 0;

  lock = open("/var/run/postgresql/totalrecall.lock", O_CREAT | O_RDONLY);
  if (lock == -1) {
    elog(FATAL, "TotalRecall: failed to open lock file");
  }
  if (flock(lock, LOCK_EX) == -1) {
    elog(FATAL, "TotalRecall: failed to obtain file lock");
  }

  // TODO: Using PGDATA isn't the right solution here.
  sprintf(filename, "%s/totalrecall.metrics", getenv("PGDATA"));
  errno = 0;
  fp = fopen(filename, "r");
  if (fp == NULL) {
    if (errno == ENOENT) {
      elog(LOG, "TotalRecall: no metrics file found");
    }
    else {
      elog(LOG, "TotalRecall: failed to open metrics file");
    }
    return;
  }

  SetCurrentStatementStartTimestamp();
  StartTransactionCommand();
  SPI_connect();
  PushActiveSnapshot(GetTransactionSnapshot());

  while (fgets(line, sizeof(line), fp)) {
    char op[8], subject[256], start_time[32], finish_time[32];
    int size;
    int ret;

    if (sscanf(line, "%10[^,],%100[^,],%100[^,],%100[^,],%d", op, subject, start_time, finish_time, &size) != 5) {
      elog(LOG, "TotalRecall: failed to read line in metrics file: %s", line);
      continue;
    }
    if (line[strlen(line) - 1] == '\n') {
      line[strlen(line) - 1] = 0;
    }

    resetStringInfo(buf);
    appendStringInfo(buf,
                     "INSERT INTO \"%s\".\"%s\" (op, subject, start_time, finish_time, size)"
                     " VALUES ('%s', '%s', '%s', '%s', %d)",
                     schema_name, table_name,
                     op, subject, start_time, finish_time, size);
    ret = SPI_execute(buf->data, false, 0);
    if (ret != SPI_OK_INSERT) {
      elog(FATAL, "TotalRecall: failed to execute insertion: %s", buf->data);
    }

    elog(LOG, "TotalRecall: ingested line: %s", line);
    ++n_ingested;
  }

  SPI_finish();
  PopActiveSnapshot();
  CommitTransactionCommand();

  elog(LOG, "TotalRecall: ingested %d metric record(s)", n_ingested);
  if (remove(filename) != 0) {
    elog(LOG, "TotalRecall: failed to delete metrics file");
  }
  fclose(fp);
}

void
initialize_schema(StringInfoData* buf)
{
  int ret, ntup;
  bool isnull;

  SetCurrentStatementStartTimestamp();
  StartTransactionCommand();
  SPI_connect();
  PushActiveSnapshot(GetTransactionSnapshot());
  pgstat_report_activity(STATE_RUNNING, "initializing TotalRecall schema");

  // TODO: Could we use CREATE SCHEMA IF NOT EXISTS?
  resetStringInfo(buf);
  appendStringInfo(buf, "SELECT COUNT(*) FROM pg_namespace WHERE nspname = '%s'", schema_name);

  debug_query_string = buf->data;
  ret = SPI_execute(buf->data, true, 0);
  if (ret != SPI_OK_SELECT) {
    elog(FATAL, "SPI_execute failed: error code %d", ret);
  }

  if (SPI_processed != 1) {
    elog(FATAL, "not a singleton result");
  }

  ntup = DatumGetInt64(SPI_getbinval(SPI_tuptable->vals[0], SPI_tuptable->tupdesc, 1, &isnull));
  if (isnull) {
    elog(FATAL, "null result");
  }

  if (ntup == 0) {
    debug_query_string = NULL;
    resetStringInfo(buf);
    appendStringInfo(buf,
                     "CREATE SCHEMA \"%s\" "
                     "CREATE TABLE \"%s\" ("
                     " id serial primary key,"
                     " op text,"
                     " subject text,"
                     " start_time timestamp,"
                     " finish_time timestamp,"
                     " size int)",
                     schema_name, table_name, table_name, table_name);

    SetCurrentStatementStartTimestamp();
    debug_query_string = buf->data;
    ret = SPI_execute(buf->data, false, 0);

    if (ret != SPI_OK_UTILITY) {
      elog(FATAL, "failed to create TotalRecall schema");
    }

    debug_query_string = NULL;	/* rest is not statement-specific */
  }

  SPI_finish();
  PopActiveSnapshot();
  CommitTransactionCommand();
  debug_query_string = NULL;
  pgstat_report_activity(STATE_IDLE, NULL);
}

void
background_main(Datum main_arg)
{
  StringInfoData buf;

  elog(LOG, "TotalRecall: starting worker");
  pqsignal(SIGTERM, die);  // TODO: This could be a problem if child is running.

  initStringInfo(&buf);

  BackgroundWorkerUnblockSignals();

  // TODO: Not sure using this envvar is enough.
  BackgroundWorkerInitializeConnection(getenv("POSTGRES_DB"), NULL, 0);
  initialize_schema(&buf);

  wait_latch(1000);  // this seems to be set the first time through, so should just get ignored
  reset_latch();
  for (;;) {
    elog(LOG, "TotalRecall: waking up");
    exec_backup();
    ingest_metrics(&buf);
    elog(LOG, "TotalRecall: going to sleep");
    wait_latch(10 * 60 * 1000);  // this seems to be set the first time through
    reset_latch();
    CHECK_FOR_INTERRUPTS();
  }
}

void
_PG_init(void)
{
  BackgroundWorker worker;

  /* DefineCustomStringVariable("totalrecall.database", */
  /*                            "Database to store metrics to.", */
  /*                            NULL, */
  /*                            &dbname, */
  /*                            "postgres", */
  /*                            PGC_POSTMASTER, */
  /*                            0, */
  /*                            NULL, NULL, NULL); */

  /* MarkGUCPrefixReserved("totalrecall"); */

  MemSet(&worker, 0, sizeof(BackgroundWorker));
  worker.bgw_flags = BGWORKER_SHMEM_ACCESS | BGWORKER_BACKEND_DATABASE_CONNECTION;
  worker.bgw_start_time = BgWorkerStart_RecoveryFinished;
  snprintf(worker.bgw_library_name, BGW_MAXLEN, "totalrecall_worker");
  snprintf(worker.bgw_function_name, BGW_MAXLEN, "background_main");
  snprintf(worker.bgw_name, BGW_MAXLEN, "TotalRecallWorker");
  worker.bgw_restart_time = 10;  // seconds
  worker.bgw_main_arg = (Datum) 0;
  worker.bgw_notify_pid = 0;
  RegisterBackgroundWorker(&worker);
}

int wait_latch(long milliseconds) {
  return WaitLatch(MyLatch,
                   WL_LATCH_SET | WL_TIMEOUT | WL_POSTMASTER_DEATH,
                   milliseconds,
                   PG_WAIT_EXTENSION);
}

void reset_latch(void) {
  return ResetLatch(MyLatch);
}

package internal

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

func Backup(scheduled bool, ctx *Context) error {
	log := ctx.Log
	log.Info("starting backup", zap.String("host", ctx.Database.Host), zap.String("dbname", ctx.Database.Name))
	startTime := time.Now()

	if scheduled {
		log.Info("checking backup schedule")
		_, closestTime := GetClosestBackupKey(time.Now(), ctx)
		if closestTime != nil {
			// TODO: Source schedule from configuration.
			if closestTime.After(time.Now().Add(-7 * 24 * time.Hour)) {
				log.Info("backup not scheduled, skipping")
				return nil
			}
		}
		log.Info("backup is scheduled, continuing")
	}

	dbUrl := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		ctx.Database.User,
		ctx.Database.Password,
		ctx.Database.Host,
		ctx.Database.Port,
		ctx.Database.Name,
	)
	cmd := exec.Command("pg_basebackup", "-d", dbUrl, "-D-", "-Ft", "-Xnone", "-r", "1024M")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("failed to open pipe from pg_basebackup", zap.Error(err))
	}
	defer stdout.Close()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		log.Fatal("failed to start pg_basebackup", zap.Error(err))
	}

	filename := makeBackupFilename()
	dst := filepath.Join(ctx.StoragePrefix, ctx.Database.Id, "backups", filename)
	size, _ := StreamToS3(stdout, dst, ctx)

	if err := cmd.Wait(); err != nil {
		if exitcode, ok := err.(*exec.ExitError); ok {
			log.Fatal(
				"pg_basebackup exited with error",
				zap.String("stderr", string(stderr.Bytes())),
				zap.Int("exit code", exitcode.ExitCode()),
			)
		} else {
			log.Fatal("failed to wait on pg_basebackup", zap.Error(err))
		}
	}

	storeMetric(&Metric{
		Op: "BA",
		Subject: filename,
		StartTime: startTime,
		FinishTime: time.Now(),
		Size: size,
	}, ctx)

	log.Info("finished backup")
	return nil
}

func makeBackupFilename() string {
	return fmt.Sprintf("%s.tar.gz", FormatTime(time.Now()))
}

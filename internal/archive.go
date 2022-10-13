package internal

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

func ArchivePut(path string, ctx *Context) error {
	log := ctx.Log
	log.Info("starting archive put", zap.String("path", path))
	startTime := time.Now()

	f, err := os.Open(path)
	if err != nil {
		log.Fatal("failed to open archive file", zap.String("path", path), zap.Error(err))
	}
	defer f.Close()

	dst := filepath.Join(ctx.StoragePrefix, ctx.Database.Id, "archive", filepath.Base(path))
	size, _ := StreamToS3(f, dst, ctx)

	storeMetric(&Metric{
		Op: "AP",
		Subject: filepath.Base(path),
		StartTime: startTime,
		FinishTime: time.Now(),
		Size: size,
	}, ctx)

	log.Info("finished archive put")
	return nil
}

func ArchiveGet(dstPath string, filename string, ctx *Context) error {
	log := ctx.Log
	log.Info("starting archive get", zap.String("dst_path", dstPath), zap.String("filename", filename))
	startTime := time.Now()

	f, err := os.Create(dstPath)
	if err != nil {
		log.Fatal("failed to open archive file", zap.String("dst_path", dstPath), zap.Error(err))
	}
	defer f.Close()

	src := filepath.Join(ctx.StoragePrefix, ctx.Database.Id, "archive", filename)
	size, _ := StreamFromS3(f, src, ctx)

	storeMetric(&Metric{
		Op: "AG",
		Subject: filename,
		StartTime: startTime,
		FinishTime: time.Now(),
		Size: size,
	}, ctx)

	log.Info("finished archive get")
	return nil
}

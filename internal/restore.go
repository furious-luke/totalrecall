package internal

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

func Restore(try bool, copyPgConf string, ctx *Context) error {
	log := ctx.Log
	log.Info("starting restore")
	startTime := time.Now()

	// TODO: Does anything exist in the destination directory? Throw a meaningful error.

	restoreTime := time.Now()
	closestBackupKey, _ := GetClosestBackupKey(restoreTime, ctx)
	if closestBackupKey == nil {
		if try {
			log.Info("no backups found, not performing restoration")
			return nil
		}
		log.Fatal("failed to find an appropriate backup")
	}
	log.Info("restoring backup", zap.Time("restore_time", restoreTime), zap.String("backup_key", *closestBackupKey))

	pipeReader, pipeWriter := io.Pipe()
	defer pipeWriter.Close()
	go func() {
		defer pipeReader.Close()
		tarReader := tar.NewReader(pipeReader)
		for {
			header, err := tarReader.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatal("failed to move to next TAR file", zap.Error(err))
			}
			filename := header.Name
			fullPath := filepath.Join(ctx.Database.DataDir, filename)
			switch header.Typeflag {
			case tar.TypeDir:
				err = os.MkdirAll(fullPath, os.FileMode(header.Mode))
				if err != nil {
					log.Fatal("failed to create directory", zap.Error(err))
				}
			case tar.TypeReg:
				fWriter, err := os.Create(fullPath)
				if err != nil {
					log.Fatal("failed to create file", zap.Error(err))
				}
				io.Copy(fWriter, tarReader)
				err = os.Chmod(fullPath, os.FileMode(header.Mode))
				if err != nil {
					log.Fatal("failed to set file permissions", zap.Error(err))
				}
				fWriter.Close()
			default:
				log.Fatal(
					"unrecognised TAR type",
					zap.String("type", string(header.Typeflag)),
					zap.String("filename", filename),
				)
			}
		}
	}()
	size, _ := StreamFromS3(pipeWriter, *closestBackupKey, ctx)

	if len(copyPgConf) > 0 {
		srcFile, err := os.Open(copyPgConf)
		if err != nil {
			log.Fatal("failed to open source PostgreSQL configuration file", zap.Error(err))
		}
		dstFile, err := os.OpenFile(
			filepath.Join(ctx.Database.DataDir, filepath.Base(copyPgConf)),
			os.O_RDWR|os.O_CREATE|os.O_TRUNC,
			os.ModePerm,
		)
		if err != nil {
			log.Fatal("failed to open destination PostgreSQL configuration file", zap.Error(err))
		}
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			log.Fatal("failed to copy PostgreSQL configuration file", zap.Error(err))
		}
	}

	signalFile, err := os.Create(filepath.Join(ctx.Database.DataDir, "recovery.signal"))
	if err != nil {
		log.Fatal("failed to create recovery signal file", zap.Error(err))
	}
	defer signalFile.Close()

	storeMetric(&Metric{
		Op: "RE",
		Subject: filepath.Base(*closestBackupKey),
		StartTime: startTime,
		FinishTime: time.Now(),
		Size: size,
	}, ctx)

	log.Info("finished backup restore")
	return nil
}

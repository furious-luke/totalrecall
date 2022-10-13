package internal

import (
	"fmt"
	"time"
	"os"
	"path/filepath"

	"github.com/alexflint/go-filemutex"
	"go.uber.org/zap"
)

type Metric struct {
	Op string
	Subject string
	StartTime time.Time
	FinishTime time.Time
	Size int64
}

func storeMetric(metric *Metric, ctx *Context) {
	log := ctx.Log
	log.Info("storing metric")

	m, err := filemutex.New("/var/run/postgresql/totalrecall.lock")
	if err != nil {
		log.Fatal("unable to obtain file lock", zap.Error(err))
	}
	m.Lock()
	defer m.Unlock()

	f, err := os.OpenFile(
		filepath.Join(ctx.Database.DataDir, "totalrecall.metrics"),
		os.O_APPEND | os.O_CREATE | os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Error("failed to open metrics file", zap.Error(err))
		return
	}
	defer f.Close()

	_, err = f.WriteString(
		fmt.Sprintf(
			"%s,%s,%s,%s,%d\n",
			metric.Op,
			metric.Subject,
			metric.StartTime.Format(time.RFC3339),
			metric.FinishTime.Format(time.RFC3339),
			metric.Size,
		),
	)
	if err != nil {
		log.Error("failed to write to metrics file", zap.Error(err))
		return
	}
}

package main

import (
	"github.com/alecthomas/kong"
	"github.com/alecthomas/kong-yaml"

	"github.com/furious-luke/totalrecall/internal"
)

type ArchivePutCmd struct {
	Path string `arg:"" name:"path" help:"WAL segment to put into the archive." type:"path"`
}

func (a *ArchivePutCmd) Run(ctx *internal.Context) error {
	return internal.ArchivePut(a.Path, ctx)
}

type ArchiveGetCmd struct {
	DstPath  string `arg:"" name:"dst-path" help:"Location to restore WAL segment." type:"path"`
	Filename string `arg:"" name:"filename" help:"WAL segment to get from the archive."`
}

func (a *ArchiveGetCmd) Run(ctx *internal.Context) error {
	return internal.ArchiveGet(a.DstPath, a.Filename, ctx)
}

type BackupCmd struct {
	Scheduled bool `help:"Skip backup if not within scheduled time."`
}

func (b *BackupCmd) Run(ctx *internal.Context) error {
	return internal.Backup(b.Scheduled, ctx)
}

type RestoreCmd struct {
	Try        bool   `help:"Don't fail if backups are missing."`
	CopyPgConf string `help:"Update PostgreSQL configuration."`
}

func (r *RestoreCmd) Run(ctx *internal.Context) error {
	return internal.Restore(r.Try, r.CopyPgConf, ctx)
}

type CliArguments struct {
	ConfigFile         kong.ConfigFlag `short:"c"`
	Debug              bool            `help:"Enable debug mode."`
	AwsAccessKeyId     string          `env:"AWS_ACCESS_KEY_ID"`
	AwsSecretAccessKey string          `env:"AWS_SECRET_ACCESS_KEY"`
	AwsRegion          string          `env:"AWS_REGION"`
	Bucket             string          ``
	StoragePrefix      string          ``
	EncryptionKey      string          ``
	Database           struct {
		Id       string ``
		Host     string `default:"localhost"`
		Port     int    `default:"5432"`
		Name     string ``
		User     string ``
		Password string ``
		DataDir  string `env:"PGDATA"`
	} `embed:"" prefix:"database-"`
	ArchivePut ArchivePutCmd `cmd:"" help:"Archive WAL segments."`
	ArchiveGet ArchiveGetCmd `cmd:"" help:"Retrieve WAL segments."`
	Backup     BackupCmd     `cmd:"" help:"Perform base backups."`
	Restore    RestoreCmd    `cmd:"" help:"Restore a backup."`
}

func main() {
	var cli CliArguments
	kongCtx := kong.Parse(
		&cli,
		kong.Configuration(kongyaml.Loader, "/etc/totalrecall/totalrecall.yaml"),
	)
	ctx := internal.Context{
		Debug:              cli.Debug,
		AwsAccessKeyId:     cli.AwsAccessKeyId,
		AwsSecretAccessKey: cli.AwsSecretAccessKey,
		AwsRegion:          cli.AwsRegion,
		StoragePrefix:      cli.StoragePrefix,
		Bucket:             cli.Bucket,
		EncryptionKey:      cli.EncryptionKey,
		Database: internal.Db{
			Id:       cli.Database.Id,
			Host:     cli.Database.Host,
			Port:     cli.Database.Port,
			Name:     cli.Database.Name,
			User:     cli.Database.User,
			Password: cli.Database.Password,
			DataDir:  cli.Database.DataDir,
		},
	}
	internal.PrepareContext(&ctx)
	err := kongCtx.Run(&ctx)
	kongCtx.FatalIfErrorf(err)
}

package internal

import (
	"os"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Db struct {
	Id       string
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	DataDir  string
}

type Context struct {
	Debug              bool
	Log                *zap.Logger
	AwsAccessKeyId     string
	AwsSecretAccessKey string
	AwsRegion          string
	Bucket             string
	StoragePrefix      string
	EncryptionKey      string
	Database           Db
}

func PrepareContext(ctx *Context) {
	ctx.AwsAccessKeyId = readSecret(ctx.AwsAccessKeyId)
	ctx.AwsSecretAccessKey = readSecret(ctx.AwsSecretAccessKey)
	ctx.AwsRegion = readSecret(ctx.AwsRegion)
	ctx.StoragePrefix = readSecret(ctx.StoragePrefix)
	ctx.Bucket = readSecret(ctx.Bucket)
	ctx.EncryptionKey = readSecret(ctx.EncryptionKey)
	ctx.Database.Id = readSecret(ctx.Database.Id)
	ctx.Database.Host= readSecret(ctx.Database.Host)
	ctx.Database.Name = readSecret(ctx.Database.Name)
	ctx.Database.User = readSecret(ctx.Database.User)
	ctx.Database.Password = readSecret(ctx.Database.Password)
	ctx.Database.DataDir = readSecret(ctx.Database.DataDir)

	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	ctx.Log , _ = logCfg.Build()

	if len(ctx.AwsAccessKeyId) == 0 || len(ctx.AwsSecretAccessKey) == 0 || len(ctx.AwsRegion) == 0 {
		fmt.Println("Please provide all AWS credentials (AWS access key ID, AWS secret key, and AWS region).")
		os.Exit(1)
	}
	if len(ctx.Bucket) == 0 {
		fmt.Println("Please provide the name of the destination AWS bucket.")
		os.Exit(1)
	}
	if len(ctx.EncryptionKey) == 0 {
		fmt.Println("Please provide an encryption key for backup encryption.")
		os.Exit(1)
	}
	if len(ctx.Database.Id) == 0 {
		fmt.Println("Please provide a database ID; note that it's important to ensure database IDs are unique")
		fmt.Println("across all databases using the same bucket and storage prefix.")
		os.Exit(1)
	}
	if len(ctx.Database.Host) == 0 || len(ctx.Database.Name) == 0 || len(ctx.Database.User) == 0 || len(ctx.Database.Password) == 0 {
		fmt.Println("Please provide all database connection paramters (host, DB name, username, and password).")
		os.Exit(1)
	}
	if len(ctx.Database.DataDir) == 0 {
		fmt.Println("Please provide the database data directory.")
		os.Exit(1)
	}
}

func readSecret(secret string) string {
	return derefSecret(derefSecret(secret))
}

func derefSecret(secret string) string {
	if strings.HasPrefix(secret, "file:") {
		path := secret[5:]
		buf, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("failed to open secret file %s", path)
			os.Exit(1)
		}
		return strings.TrimSpace(string(buf))
	} else if strings.HasPrefix(secret, "env:") {
		env := secret[4:]
		return os.Getenv(env)
	} else {
		return secret
	}
}

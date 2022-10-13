package internal

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

func StreamToS3(input io.Reader, dst string, ctx *Context) (int64, error) {
	log := ctx.Log

	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()

	iv := make([]byte, aes.BlockSize)
	rand.Read(iv)

	sizeResult := make(chan int64, 1)
	go func(result chan int64) {
		defer pipeWriter.Close()

		key := []byte(ctx.EncryptionKey)
		block, err := aes.NewCipher(key)
		if err != nil {
			log.Fatal("failed to create AES cipher", zap.Error(err))
		}
		stream := cipher.NewOFB(block, iv)
		encryptWriter := &cipher.StreamWriter{S: stream, W: pipeWriter}
		defer encryptWriter.Close()

		compressWriter := gzip.NewWriter(encryptWriter)

		nRead, err := io.Copy(compressWriter, input)
		if err != nil {
			log.Fatal("failed to copy source to output stream", zap.Error(err))
		}
		if err := compressWriter.Close(); err != nil {
			log.Fatal("failed to complete compression", zap.Error(err))
		}

		sizeResult <- nRead
	}(sizeResult)

	client := makeS3Client(ctx)
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024 // the minimum/default allowed part size is 5MB
		u.Concurrency = 1            // default is 5
	})
	result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &ctx.Bucket,
		Key:    &dst,
		Body:   io.MultiReader(bytes.NewReader(iv), pipeReader),
	})
	if err != nil {
		log.Fatal("upload to s3 failed", zap.Error(err))
	}
	log.Info("file uploaded to s3",
		zap.String("location", result.Location),
		zap.Int("n_parts", len(result.CompletedParts)),
	)

	return <-sizeResult, nil
}

type SequentialWriter struct {
	w io.Writer
}

func (sw SequentialWriter) WriteAt(p []byte, offset int64) (n int, err error) {
	// TODO: Assert linear offsets.
	return sw.w.Write(p)
}

func StreamFromS3(output io.Writer, src string, ctx *Context) (int64, error) {
	log := ctx.Log

	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()

	go func() {
		defer pipeWriter.Close()
		client := makeS3Client(ctx)
		downloader := manager.NewDownloader(client, func(d *manager.Downloader) {
			d.PartSize = 5 * 1024 * 1024 // the minimum/default allowed part size is 5MB
			d.Concurrency = 1            // default is 5
		})
		_, err := downloader.Download(context.TODO(), SequentialWriter{w: pipeWriter}, &s3.GetObjectInput{
			Bucket: &ctx.Bucket,
			Key:    &src,
		})
		if err != nil {
			log.Fatal("download from s3 failed", zap.Error(err))
		}
	}()

	var iv [aes.BlockSize]byte
	nRead, err := io.ReadFull(pipeReader, iv[:])
	if err != nil || nRead != aes.BlockSize {
		log.Fatal("failed to read IV", zap.Error(err))
	}

	key := []byte(ctx.EncryptionKey)
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal("failed to create AES cipher", zap.Error(err))
	}
	stream := cipher.NewOFB(block, iv[:])
	decryptReader := &cipher.StreamReader{S: stream, R: pipeReader}

	decompressReader, err := gzip.NewReader(decryptReader)
	if err != nil {
		log.Fatal("failed to create gzip reader", zap.Error(err))
	}
	defer decompressReader.Close()

	size, err := io.Copy(output, decompressReader)
	if err != nil {
		log.Fatal("failed to copy source to output stream", zap.Error(err))
	}

	return size, nil
}

func GetClosestBackupKey(restoreTime time.Time, ctx *Context) (*string, *time.Time) {
	log := ctx.Log

	client := makeS3Client(ctx)
	prefix := filepath.Join(ctx.StoragePrefix, ctx.Database.Id, "backups")
	var closestKey *string = nil
	var closestTime *time.Time = nil
	for {
		response, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket:     &ctx.Bucket,
			Prefix:     &prefix,
			StartAfter: closestKey,
		})
		if err != nil {
			log.Fatal("failed to retrieve S3 listing", zap.Error(err))
		}
		for _, obj := range response.Contents {
			filename := filepath.Base(*obj.Key)
			objTime, err := ParseTime(filename[:len(filename)-7])
			if err != nil {
				log.Fatal("failed to parse backup time", zap.Error(err))
			}
			if objTime.After(restoreTime) {
				return closestKey, closestTime
			}
			closestKey = obj.Key
			closestTime = &objTime
		}
		if !response.IsTruncated {
			return closestKey, closestTime
		}
	}
}

func makeS3Client(ctx *Context) *s3.Client {
	log := ctx.Log

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(ctx.AwsAccessKeyId, ctx.AwsSecretAccessKey, ""),
		),
		config.WithRegion(ctx.AwsRegion),
	)
	if err != nil {
		log.Fatal("failed to create S3 client")
	}
	return s3.NewFromConfig(cfg)
}

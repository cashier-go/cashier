package s3fs

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"go4.org/wkfs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nsheridan/cashier/server/config"
)

// Register the /s3/ filesystem as a well-known filesystem.
func Register(config *config.AWS) {
	if config == nil {
		registerBrokenFS(errors.New("aws credentials not found"))
		return
	}
	ac := &aws.Config{}
	// If region is unset the SDK will attempt to read the region from the environment.
	if config.Region != "" {
		ac.Region = aws.String(config.Region)
	}
	// Attempt to get credentials from the cashier config.
	// Otherwise check for standard credentials. If neither are present register the fs as broken.
	// TODO: implement this as a provider.
	if config.AccessKey != "" && config.SecretKey != "" {
		ac.Credentials = credentials.NewStaticCredentials(config.AccessKey, config.SecretKey, "")
	} else {
		_, err := session.New().Config.Credentials.Get()
		if err != nil {
			registerBrokenFS(errors.New("aws credentials not found"))
			return
		}
	}
	sc := s3.New(session.New(ac))
	if aws.StringValue(sc.Config.Region) == "" {
		registerBrokenFS(errors.New("aws region configuration not found"))
		return
	}
	wkfs.RegisterFS("/s3/", &s3FS{
		sc: sc,
	})
}

func registerBrokenFS(err error) {
	wkfs.RegisterFS("/s3/", &s3FS{
		err: err,
	})
}

type s3FS struct {
	sc  *s3.S3
	err error
}

func (fs *s3FS) parseName(name string) (bucket, fileName string, err error) {
	if fs.err != nil {
		return "", "", fs.err
	}
	name = strings.TrimPrefix(name, "/s3/")
	i := strings.Index(name, "/")
	if i < 0 {
		return name, "", nil
	}
	return name[:i], name[i+1:], nil
}

// Open opens the named file for reading.
func (fs *s3FS) Open(name string) (wkfs.File, error) {
	bucket, fileName, err := fs.parseName(name)
	if err != nil {
		return nil, err
	}
	obj, err := fs.sc.GetObject(&s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &fileName,
	})
	if err != nil {
		return nil, err
	}
	defer obj.Body.Close()
	slurp, err := ioutil.ReadAll(obj.Body)
	if err != nil {
		return nil, err
	}
	return &file{
		name:   name,
		Reader: bytes.NewReader(slurp),
	}, nil
}

func (fs *s3FS) Stat(name string) (os.FileInfo, error) { return fs.Lstat(name) }
func (fs *s3FS) Lstat(name string) (os.FileInfo, error) {
	bucket, fileName, err := fs.parseName(name)
	if err != nil {
		return nil, err
	}
	obj, err := fs.sc.GetObject(&s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &fileName,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "NoSuchKey" {
				return nil, os.ErrNotExist
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return &statInfo{
		name: path.Base(fileName),
		size: *obj.ContentLength,
	}, nil
}

func (fs *s3FS) MkdirAll(path string, perm os.FileMode) error { return nil }

func (fs *s3FS) OpenFile(name string, flag int, perm os.FileMode) (wkfs.FileWriter, error) {
	return nil, errors.New("not implemented")
}

type statInfo struct {
	name    string
	size    int64
	isDir   bool
	modtime time.Time
}

func (si *statInfo) IsDir() bool        { return si.isDir }
func (si *statInfo) ModTime() time.Time { return si.modtime }
func (si *statInfo) Mode() os.FileMode  { return 0644 }
func (si *statInfo) Name() string       { return path.Base(si.name) }
func (si *statInfo) Size() int64        { return si.size }
func (si *statInfo) Sys() interface{}   { return nil }

type file struct {
	name string
	*bytes.Reader
}

func (*file) Close() error   { return nil }
func (f *file) Name() string { return path.Base(f.name) }
func (f *file) Stat() (os.FileInfo, error) {
	panic("Stat not implemented on /s3/ files yet")
}

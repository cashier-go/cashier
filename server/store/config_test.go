package store

import (
	"reflect"
	"testing"
	"time"

	mgo "gopkg.in/mgo.v2"
)

func TestMySQLConfig(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"mysql:user:passwd:localhost", "user:passwd@tcp(localhost:3306)/certs?parseTime=true"},
		{"mysql:user:passwd:localhost:13306", "user:passwd@tcp(localhost:13306)/certs?parseTime=true"},
		{"mysql:root::localhost", "root@tcp(localhost:3306)/certs?parseTime=true"},
	}
	for _, tt := range tests {
		result := parseMySQLConfig(tt.in)
		if result != tt.out {
			t.Errorf("want %s, got %s", tt.out, result)
		}
	}
}

func TestMongoConfig(t *testing.T) {
	var tests = []struct {
		in  string
		out *mgo.DialInfo
	}{
		{"mongo:user:passwd:host", &mgo.DialInfo{
			Username: "user",
			Password: "passwd",
			Addrs:    []string{"host"},
			Database: "certs",
			Timeout:  5 * time.Second,
		}},
		{"mongo:user:passwd:host1,host2", &mgo.DialInfo{
			Username: "user",
			Password: "passwd",
			Addrs:    []string{"host1", "host2"},
			Database: "certs",
			Timeout:  5 * time.Second,
		}},
		{"mongo:user:passwd:host1:27017,host2:27017", &mgo.DialInfo{
			Username: "user",
			Password: "passwd",
			Addrs:    []string{"host1:27017", "host2:27017"},
			Database: "certs",
			Timeout:  5 * time.Second,
		}},
		{"mongo:user:passwd:host1,host2:27017", &mgo.DialInfo{
			Username: "user",
			Password: "passwd",
			Addrs:    []string{"host1", "host2:27017"},
			Database: "certs",
			Timeout:  5 * time.Second,
		}},
	}
	for _, tt := range tests {
		result := parseMongoConfig(tt.in)
		if !reflect.DeepEqual(result, tt.out) {
			t.Errorf("want:\n%+v\ngot:\n%+v", tt.out, result)
		}
	}
}

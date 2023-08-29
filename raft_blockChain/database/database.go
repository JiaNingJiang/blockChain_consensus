package database

import (
	"encoding/json"
	"sync"

	"github.com/hashicorp/raft"
)

type Database struct {
	Data map[string]string
	mu   sync.Mutex
}

func NewDatabase() *Database {
	return &Database{
		Data: make(map[string]string),
	}
}

func (d *Database) Get(key string) string {
	d.mu.Lock()
	value := d.Data[key]
	d.mu.Unlock()
	return value
}

func (d *Database) Set(key, value string) {
	d.mu.Lock()
	d.Data[key] = value
	d.mu.Unlock()
}

func (d *Database) Persist(sink raft.SnapshotSink) error {
	d.mu.Lock()
	data, err := json.Marshal(d.Data)
	d.mu.Unlock()
	if err != nil {
		return err
	}
	sink.Write(data)
	sink.Close()
	return nil
}

func (d *Database) Release() {}

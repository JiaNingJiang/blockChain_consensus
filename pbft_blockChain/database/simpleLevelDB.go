package database

import "github.com/syndtr/goleveldb/leveldb"

type SimpleLDB struct {
	name string      // the name of this db
	db   *leveldb.DB // the instance of LevelDB
}

func NewSimpleLDB(name string, db *leveldb.DB) *SimpleLDB {
	sldb := &SimpleLDB{
		name: name,
		db:   db,
	}
	return sldb
}

func (sldb *SimpleLDB) Put(key []byte, value []byte) error {
	return sldb.db.Put(key, value, nil)
}

func (sldb *SimpleLDB) Get(key []byte) ([]byte, error) {
	dat, err := sldb.db.Get(key, nil)
	return dat, err

}

func (sldb *SimpleLDB) Delete(key []byte) error {
	return sldb.db.Delete(key, nil)
}

func (sldb *SimpleLDB) Close() {
	sldb.db.Close()
}

func (sldb *SimpleLDB) Start() {
	panic("not implemented") // TODO: Implement
}

func (sldb *SimpleLDB) Flush() error {
	panic("not implemented") // TODO: Implement
}

package database

import (
	"pbft_blockchain/metrics"
	"strconv"
	"strings"
	"sync"
	"time"

	gometrics "github.com/rcrowley/go-metrics"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var CacheLimit = 64

type LDBDatabase struct {
	fn string      // file name of DB
	db *leveldb.DB // the instance of LevelDB

	getTimer       gometrics.Timer // Timer for measuring the database get request counts and latencies
	putTimer       gometrics.Timer // Timer for measuring the database put request counts and latencies
	delTimer       gometrics.Timer // Timer for measuring the database delete request counts and latencies
	missMeter      gometrics.Meter // Meter for measuring the missed database get requests
	readMeter      gometrics.Meter // Meter for measuring the database get request data usage
	writeMeter     gometrics.Meter // Meter for measuring the database put request data usage
	compTimeMeter  gometrics.Meter // Meter for measuring the total time spent in database compaction
	compReadMeter  gometrics.Meter // Meter for measuring the data read during compaction
	compWriteMeter gometrics.Meter // Meter for measuring the data written during compaction

	quitLock sync.Mutex      //Mutex protecting the quit channel access
	quitChan chan chan error //Quit channel to stop the metrics collection before closing the database
}

// NewLDBDatabase returns a LevelDB wrapped object. LDBDatabase does not persist data by
// it self but requires a background poller which syncs every X. `Flush` should be called
// when data needs to be stored and written to disk.
func NewLDBDatabase(file string) (*LDBDatabase, error) {
	// Open the db
	// db, err := leveldb.OpenFile(file, &opt.Options{OpenFilesCacheCapacity: CacheLimit, NoSync: true, Compression: opt.SnappyCompression})
	db, err := leveldb.OpenFile(file, &opt.Options{OpenFilesCacheCapacity: CacheLimit, NoSync: true})
	// check for corruption and attempt to recover
	if _, iscorrupted := err.(*errors.ErrCorrupted); iscorrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}
	// (re) check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}
	return &LDBDatabase{
		fn: file,
		db: db,
	}, nil
}

// Put puts the given key / value to the queue
func (self *LDBDatabase) Put(key []byte, value []byte) error {
	// Measure the database put latency, if requested
	if self.putTimer != nil {
		defer self.putTimer.UpdateSince(time.Now())
	}
	// Generate the data to write to disk, update the meter and write
	//value = rle.Compress(value)

	if self.writeMeter != nil {
		self.writeMeter.Mark(int64(len(value)))
	}
	return self.db.Put(key, value, nil)
}

// Get returns the given key if it's present.
func (self *LDBDatabase) Get(key []byte) ([]byte, error) {

	// snap, err := self.NewSnapshot()
	// if err != nil {
	// 	loglogrus.Log.Debugf("Create New SnapShot is failed!!")
	// 	return nil, err
	// }

	// Measure the database get latency, if requested
	if self.getTimer != nil {
		defer self.getTimer.UpdateSince(time.Now())
	}
	// Retrieve the key and increment the miss counter if not found
	// dat, err := snap.Get(key)
	// if err != nil {
	// 	loglogrus.Log.Debugf("Get Data from Snap is failed!!")
	// }
	dat, err := self.db.Get(key, nil)
	if err != nil {
		if self.missMeter != nil {
			self.missMeter.Mark(1)
		}
		return nil, err
	}
	// Otherwise update the actually retrieved amount of data
	if self.readMeter != nil {
		self.readMeter.Mark(int64(len(dat)))
	}
	return dat, nil
	//return rle.Decompress(dat)
}

// Delete deletes the key from the queue and database
func (self *LDBDatabase) Delete(key []byte) error {
	// Measure the database delete latency, if requested
	if self.delTimer != nil {
		defer self.delTimer.UpdateSince(time.Now())
	}
	// Execute the actual operation
	return self.db.Delete(key, nil)
}

func (self *LDBDatabase) NewIterator() iterator.Iterator {
	return self.db.NewIterator(nil, nil)
}

// Flush flushes out the queue to leveldb
func (self *LDBDatabase) Flush() error {
	return nil
}

// Start uses the settings to load and start the database
func (slef *LDBDatabase) Start() {

}

func (self *LDBDatabase) Close() {
	// Stop the metrics collection to avoid internal database races
	self.quitLock.Lock()
	defer self.quitLock.Unlock()

	if self.quitChan != nil {
		errc := make(chan error)
		self.quitChan <- errc

	}
	// Flush and close the database
	self.db.Close()
}

func (self *LDBDatabase) LDB() *leveldb.DB {
	return self.db
}

// Meter configures the database metrics collectors and
func (self *LDBDatabase) Meter(prefix string) {
	// Initialize all the metrics collector at the requested prefix
	self.getTimer = metrics.NewTimer(prefix + "user/gets")
	self.putTimer = metrics.NewTimer(prefix + "user/puts")
	self.delTimer = metrics.NewTimer(prefix + "user/dels")
	self.missMeter = metrics.NewMeter(prefix + "user/misses")
	self.readMeter = metrics.NewMeter(prefix + "user/reads")
	self.writeMeter = metrics.NewMeter(prefix + "user/writes")
	self.compTimeMeter = metrics.NewMeter(prefix + "compact/time")
	self.compReadMeter = metrics.NewMeter(prefix + "compact/input")
	self.compWriteMeter = metrics.NewMeter(prefix + "compact/output")

	// Create a quit channel for the periodic collector and run it
	self.quitLock.Lock()
	self.quitChan = make(chan chan error)
	self.quitLock.Unlock()

	go self.meter(3 * time.Second)
}

// meter periodically retrieves internal leveldb counters and reports them to
// the metrics subsystem.
//
// This is how a stats table look like (currently):
//
//	Compactions
//	 Level |   Tables   |    Size(MB)   |    Time(sec)  |    Read(MB)   |   Write(MB)
//	-------+------------+---------------+---------------+---------------+---------------
//	   0   |          0 |       0.00000 |       1.27969 |       0.00000 |      12.31098
//	   1   |         85 |     109.27913 |      28.09293 |     213.92493 |     214.26294
//	   2   |        523 |    1000.37159 |       7.26059 |      66.86342 |      66.77884
//	   3   |        570 |    1113.18458 |       0.00000 |       0.00000 |       0.00000
func (self *LDBDatabase) meter(refresh time.Duration) {
	// Create the counters to store current and previous values
	counters := make([][]float64, 2)
	for i := 0; i < 2; i++ {
		counters[i] = make([]float64, 3)
	}
	// Iterate ad infinitum and collect the stats
	for i := 1; ; i++ {
		// Retrieve the database stats
		stats, err := self.db.GetProperty("leveldb.stats")
		if err != nil {
			return
		}
		// Find the compaction table, skip the header
		lines := strings.Split(stats, "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[0]) != "Compactions" {
			lines = lines[1:]
		}
		if len(lines) <= 3 {
			return
		}
		lines = lines[3:]

		// Iterate over all the table rows, and accumulate the entries
		for j := 0; j < len(counters[i%2]); j++ {
			counters[i%2][j] = 0
		}
		for _, line := range lines {
			parts := strings.Split(line, "|")
			if len(parts) != 6 {
				break
			}
			for idx, counter := range parts[3:] {
				if value, err := strconv.ParseFloat(strings.TrimSpace(counter), 64); err != nil {
					return
				} else {
					counters[i%2][idx] += value
				}
			}
		}
		// Update all the requested meters
		if self.compTimeMeter != nil {
			self.compTimeMeter.Mark(int64((counters[i%2][0] - counters[(i-1)%2][0]) * 1000 * 1000 * 1000))
		}
		if self.compReadMeter != nil {
			self.compReadMeter.Mark(int64((counters[i%2][1] - counters[(i-1)%2][1]) * 1024 * 1024))
		}
		if self.compWriteMeter != nil {
			self.compWriteMeter.Mark(int64((counters[i%2][2] - counters[(i-1)%2][2]) * 1024 * 1024))
		}
		// Sleep a bit, then repeat the stats collection
		select {
		case errc := <-self.quitChan:
			// Quit requesting, stop hammering the database
			errc <- nil
			return

		case <-time.After(refresh):
			// Timeout, gather a new set of stats
		}
	}
}

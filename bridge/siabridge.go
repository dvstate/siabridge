// Written by David Gore
// No restrictions on use, providing that the code is used exclusively to interact with Sia network.
// Permission is not granted to adapt this code for use on other storage backends.

package bridge

import (
	"fmt"
	"time"
	"os"
	"io"
	"path/filepath"
	"errors"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/NebulousLabs/Sia/api"
)

// How many seconds to delay between cache/db management operations
const MANAGER_DELAY_SEC = 30

// Global ticker for cache management
var g_cache_ticker *time.Ticker

// Global database object
var g_db *sql.DB

type SiaBridge struct {
	SiadAddress string 	// Address of siad daemon API. (e.g., "127.0.0.1:9980")
	CacheDir string 	// Cache directory for downloads
	DbFile string 		// Name and path of Sqlite database file
}

type BucketInfo struct {
	Name string 		// Name of bucket
	Created time.Time   // Time of bucket creation
}

type ObjectInfo struct {
	Bucket string 		// Name of bucket object is stored in
	Name string 		// Name of object
	Size int64			// Size of the object in bytes
	Queued time.Time 	// Time object was queued for upload to Sia
	Uploaded time.Time 	// Time object was successfully uploaded to Sia
	PurgeAfter int64    // If no downloads in this many seconds, purge from cache.
	                    // Always in cache if value is 0.
	CachedFetches int64	// The total number of times the object has been fetched from cache
	SiaFetches int64 	// The total number of times the object has been fetched from Sia network
	LastFetch time.Time // The time of the last fetch request for the object
}

// Called to start running the SiaBridge
func (b *SiaBridge) Start() error {
	// Make sure cache directory exists
	os.Mkdir(b.CacheDir, 0744)

	// Open and initialize database
	err := b.initDatabase()
	if err != nil {
		return err
	}

	// Start the cache management process
	g_cache_ticker = time.NewTicker(time.Second * MANAGER_DELAY_SEC)
    go func() {
        for _ = range g_cache_ticker.C {
        	b.manager()
        }
    }()

    return nil
}

// Called to stop the SiaBridge
func (b *SiaBridge) Stop() {
	// Stop cache management process
	g_cache_ticker.Stop()

	// Close the database
	g_db.Close()
}

// Creates a new bucket for storing objectserror
func (b *SiaBridge) CreateBucket(bucket string) error {
	// If bucket already exists, return success
	exists, err := b.bucketExists(bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil // Bucket exists; done
	}

	// Bucket doesn't exist. Create it.
	err = b.insertBucket(bucket)
	return err
}

// Returns info for the provided bucket
func (b *SiaBridge) GetBucketInfo(bucket string) (bi BucketInfo, e error) {
	// Query the database
	var created int64
	err := g_db.QueryRow("SELECT created FROM buckets WHERE name=?", bucket).Scan(&created)
	switch {
	case err == sql.ErrNoRows:
	   return bi, errors.New("Bucket does not exist")
	case err != nil:
		// An error occured
	    return bi, err 		
	default:
		// Bucket exists
		bi.Name = bucket
		bi.Created = time.Unix(created,0)
		return bi, nil
	}

	// Shouldn't happen, but just in case
	return bi, errors.New("Unknown error in GetBucketInfo()")
}

// List all buckets
func (b *SiaBridge) ListBuckets() (buckets []BucketInfo, e error) {
	rows, err := g_db.Query("SELECT * FROM buckets")
    if err != nil {
    	return buckets, err
    }

    var name string
    var created int64

    for rows.Next() {
        err = rows.Scan(&name, &created)
        if err != nil {
        	return buckets, err
        }

        buckets = append(buckets, BucketInfo{
    		Name:		name,
    		Created: 	time.Unix(created, 0),
    	})
    }

    rows.Close()

	return buckets, nil
}

// Delete a bucket, as well as all contents of the bucket
func (b *SiaBridge) DeleteBucket(bucket string) error {
	stmt, err := g_db.Prepare("DELETE FROM buckets WHERE name=?")
    if err != nil {
    	return err
    }
	_, err = stmt.Exec(bucket)
    if err != nil {
    	return err
    }

	return nil
}

// Returns a list of objects in the bucket provided
func (b *SiaBridge) ListObjects(bucket string) (objects []ObjectInfo, e error) {
	rows, err := g_db.Query("SELECT name,size,queued,uploaded,purge_after,cached_fetches,sia_fetches,last_fetch FROM objects WHERE bucket=?",bucket)
    if err != nil {
    	return objects, err
    }

    var name string
    var size int64
    var queued int64
    var uploaded int64
    var purge_after int64
    var cached_fetches int64
    var sia_fetches int64
    var last_fetch int64

    for rows.Next() {
        err = rows.Scan(&name, &size, &queued, &uploaded, &purge_after, &cached_fetches, &sia_fetches, &last_fetch)
        if err != nil {
        	return objects, err
        }

        objects = append(objects, ObjectInfo{
        	Bucket:			bucket,
    		Name:			name,
    		Size:       	size,
    		Queued:         time.Unix(queued, 0),
    		Uploaded: 		time.Unix(uploaded, 0),
    		PurgeAfter:		purge_after,
    		CachedFetches:	cached_fetches,
    		SiaFetches:		sia_fetches,
    		LastFetch:  	time.Unix(last_fetch, 0),
    	})
    }

    rows.Close()

	return objects, nil
}

// Returns info for the provided object
func (b *SiaBridge) GetObjectInfo(bucket string, objectName string) (objInfo ObjectInfo, e error) {
	// Query the database
	var size int64
	var queued int64
	var uploaded int64
	var purge_after int64
	var cached_fetches int64
	var sia_fetches int64
	var last_fetch int64
	err := g_db.QueryRow("SELECT size,queued,uploaded,purge_after,cached_fetches,sia_fetches,last_fetch FROM objects WHERE name=? AND bucket=?", objectName, bucket).Scan(&size,&queued,&uploaded,&purge_after,&cached_fetches,&sia_fetches,&last_fetch)
	switch {
	case err == sql.ErrNoRows:
		return objInfo, errors.New("Object does not exist in bucket")
	case err != nil:
		// An error occured
		return objInfo, err
	default:
		// Object exists
		objInfo.Bucket = bucket
		objInfo.Name = objectName
		objInfo.Size = size
		objInfo.Queued = time.Unix(queued,0)
		objInfo.Uploaded = time.Unix(uploaded,0)
		objInfo.PurgeAfter = purge_after
		objInfo.CachedFetches = cached_fetches
		objInfo.SiaFetches = sia_fetches
		objInfo.LastFetch = time.Unix(last_fetch,0)
		return objInfo, nil 	
	}

	// Shouldn't happen, but just in case
	return objInfo, errors.New("Unknown error in GetObjectInfo()")
}

// Writes the object identified by the bucket and object name to the writer provided
func (b *SiaBridge) GetObject(bucket string, objectName string, writer io.Writer) error {
	// Make sure object exists in database
	objInfo, err := b.GetObjectInfo(bucket, objectName)
	if err != nil {
		return err
	}

	// Prefer to deliver object from cache if available.
	// This avoids Sia network fees and excess latency.
	var siaObj = bucket + "/" + objectName
	var cachedFile = filepath.Join(b.CacheDir,siaObj)
	if _, err := os.Stat(cachedFile); err == nil {
    	reader, err := os.Open(cachedFile)
		if err != nil {
		 	return err
		}

		_, err = io.Copy(writer, reader)
		reader.Close()
    	if err != nil {
        	return err
    	}

    	// Increment cached fetch count
    	err = b.updateCachedFetches(bucket, objectName, objInfo.CachedFetches+1)
    	return err
    }

    // Object not in cache, must download from Sia.
    // First, though, make sure the file was completely uploaded to Sia.
    if objInfo.Uploaded == time.Unix(0,0) {
    	// File never completed uploaded, or was never marked as uploaded in database
    	return errors.New("Attempting to download incomplete file from Sia")
    }

    // Make sure bucket path exists in cache directory
	os.Mkdir(filepath.Join(b.CacheDir, bucket), 0744)

	err = get(b.SiadAddress, "/renter/download/" + siaObj + "?destination=" + abs(cachedFile))
	if err != nil {
		return err
	}

	reader, err := os.Open(abs(cachedFile))
    if err != nil {
        return err
    }

    _, err = io.Copy(writer, reader)
    reader.Close()
    if err != nil {
        return err
    }

    // Increment sia fetch count
	err = b.updateCachedFetches(bucket, objectName, objInfo.CachedFetches+1)
	return err
}

// Uploads the data from the io.Reader to the bucket and object name specified
func (b *SiaBridge) PutObjectFromReader(data io.Reader, bucket string, objectName string, size int64, purge_after int64) error {
	// Make sure an object of same name doesn't already exist in bucket
	exists, err := b.objectExists(bucket, objectName)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("Object with same name already exists in bucket")
	}

	// Copy the file to cache directory for Sia upload
	var siaObj = bucket + "/" + objectName
    var tmpPath = filepath.Join(b.CacheDir, siaObj)

    // Make sure bucket path exists
	os.Mkdir(filepath.Join(b.CacheDir, bucket), 0744)

	err = copyFile(data, abs(tmpPath))
	if err != nil {
		return err
	}

	// Create a database entry for the object
	err = b.insertObject(bucket, objectName, size, time.Now().Unix(), 0, purge_after)
	if err != nil {
		return err
	}

	// Tell Sia daemon to upload the object
	err = post(b.SiadAddress, "/renter/upload/"+siaObj, "source="+abs(tmpPath))
	if err != nil {
		return err
	}

	return nil
}

// Uploads the data from the file specified to the bucket and object name specified
func (b *SiaBridge) PutObjectFromFile(file string, bucket string, objectName string, purge_after int64) error {
	// Make sure file exists and get size in bytes
	fi, err := os.Stat(file);
	if err != nil {
	    return err
	}
	size := fi.Size()

	// Get a reader to the file
	data, err := os.Open(file)
	if err != nil {
		return err
	}

	err = b.PutObjectFromReader(data, bucket, objectName, size, purge_after)
	data.Close()
	return err
}

// Deletes the object
func (b *SiaBridge) DeleteObject(bucket string, objectName string) error {
	// Delete record from database
	stmt, err := g_db.Prepare("DELETE FROM objects WHERE bucket=? AND name=?")
    if err != nil {
    	return err
    }
	_, err = stmt.Exec(bucket, objectName)
    if err != nil {
    	return err
    }

    // Tell Sia daemon to delete the object
	var siaObj = bucket + "/" + objectName
	
	err = post(b.SiadAddress, "/renter/delete/"+siaObj, "")
	if err != nil {
		return err
	}

	return nil
}

// Runs periodically to manage the database and cache
func (b *SiaBridge) manager() {
	// Check to see if any files in database have completed uploading to Sia.
	// If so, update uploaded timestamp in database.
	err := b.checkSiaUploads()
	if err != nil {
		fmt.Println("Error in DB/Cache Management Process:")
		fmt.Println(err)
	}

	// Remove files from cache that have not been uploaded or fetched in purge_after seconds.
	err = b.purgeCache()
	if err != nil {
		fmt.Println("Error in DB/Cache Management Process:")
		fmt.Println(err)
	}

}

func (b *SiaBridge) purgeCache() error {
	buckets, err := b.ListBuckets()
	if err != nil {
		return err
	}

	for _, bucket := range buckets {
		objects, err := b.ListObjects(bucket.Name)
		if err != nil {
			return err
		}

		for _, object := range objects {
			if object.Uploaded != time.Unix(0,0) {
				since_uploaded := time.Now().Unix() - object.Uploaded.Unix()
				since_fetched := time.Now().Unix() - object.LastFetch.Unix()
				if since_uploaded > object.PurgeAfter && since_fetched > object.PurgeAfter {
					var siaObj = object.Bucket + "/" + object.Name
					var cachedFile = filepath.Join(b.CacheDir,siaObj)
					os.Remove(abs(cachedFile))
				}
			}
		}
	}
	return nil
}

func (b *SiaBridge) checkSiaUploads() error {
	// Get list of all uploading objects
	objs, err := b.listUploadingObjects()
	if err != nil {
		return err
	}

	// Get list of all renter files
	var rf api.RenterFiles
	err = getAPI(b.SiadAddress, "/renter/files", &rf)
	if err != nil {
		return err
	}

	// If uploading object is available on Sia, update database
	for _, obj := range objs {
		var siaObj = obj.Bucket + "/" + obj.Name
		for _, file := range rf.Files {
			if file.SiaPath == siaObj && file.Available {
				err = b.markObjectUploaded(obj.Bucket, obj.Name)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (b *SiaBridge) markObjectUploaded(bucket string, objectName string) error {
	stmt, err := g_db.Prepare("UPDATE objects SET uploaded=? WHERE bucket=? AND name=?")
    if err != nil {
    	return err
    }

    _, err = stmt.Exec(time.Now().Unix(), bucket, objectName)
    if err != nil {
    	return err
    }

    return nil
}

func (b *SiaBridge) initDatabase() error {
	// Open the database
	var e error
	g_db, e = sql.Open("sqlite3", b.DbFile)
	if e != nil {
		return e
	}

	// Make sure buckets table exists
	stmt, err := g_db.Prepare("CREATE TABLE IF NOT EXISTS buckets(name TEXT PRIMARY KEY, created INTEGER)")
    if err != nil {
    	return err
    }
	_, err = stmt.Exec()
    if err != nil {
    	return err
    }

	// Make sure objects table exists
    stmt, err = g_db.Prepare("CREATE TABLE IF NOT EXISTS objects(bucket TEXT, name TEXT, size INTEGER, queued INTEGER, uploaded INTEGER, purge_after INTEGER, cached_fetches INTEGER, sia_fetches INTEGER, last_fetch INTEGER, PRIMARY KEY(bucket,name) )")
    if err != nil {
    	return err
    }
	_, err = stmt.Exec()
    if err != nil {
    	return err
    }

	return nil
}

func (b *SiaBridge) bucketExists(bucket string) (exists bool, e error) {
	// Query the database
	var name string
	err := g_db.QueryRow("SELECT name FROM buckets WHERE name=?", bucket).Scan(&name)
	switch {
	case err == sql.ErrNoRows:
	   return false, nil		// Bucket does not exist
	case err != nil:
	   return false, err 		// An error occured
	default:
		if name == bucket {
			return true, nil 	// Bucket exists
		}
	}

	// Shouldn't happen, but just in case
	return false, errors.New("Unknown error in bucketExists()")	
}

func (b *SiaBridge) objectExists(bucket string, objectName string) (exists bool, e error) {
	// Query the database
	var bkt string
	var name string
	err := g_db.QueryRow("SELECT bucket,name FROM objects WHERE bucket=? AND name=?", 
							bucket, objectName).Scan(&bkt,&name)
	switch {
	case err == sql.ErrNoRows:
	   return false, nil		// Bucket does not exist
	case err != nil:
	   return false, err 		// An error occured
	default:
		if bkt == bucket && name == objectName {
			return true, nil 	// Object exists
		}
	}

	// Shouldn't happen, but just in case
	return false, errors.New("Unknown error in objectExists()")
}

func (b *SiaBridge) updateCachedFetches(bucket string, objectName string, fetches int64) error {
	stmt, err := g_db.Prepare("UPDATE objects SET cached_fetches=? WHERE bucket=? AND name=?")
    if err != nil {
    	return err
    }

    _, err = stmt.Exec(fetches, bucket, objectName)
    if err != nil {
    	return err
    }

    return nil
}

func (b *SiaBridge) updateSiaFetches(bucket string, objectName string, fetches int64) error {
	stmt, err := g_db.Prepare("UPDATE objects SET sia_fetches=? WHERE bucket=? AND name=?")
    if err != nil {
    	return err
    }

    _, err = stmt.Exec(fetches, bucket, objectName)
    if err != nil {
    	return err
    }

    return nil
}

func (b *SiaBridge) listUploadingObjects() (objects []ObjectInfo, e error) {
	rows, err := g_db.Query("SELECT bucket,name,size,queued,purge_after,cached_fetches,sia_fetches,last_fetch FROM objects WHERE uploaded=0")
    if err != nil {
    	return objects, err
    }

    var bucket string
    var name string
    var size int64
    var queued int64
    var purge_after int64
    var cached_fetches int64
    var sia_fetches int64
    var last_fetch int64

    for rows.Next() {
        err = rows.Scan(&bucket, &name, &size, &queued, &purge_after, &cached_fetches, &sia_fetches, &last_fetch)
        if err != nil {
        	return objects, err
        }

        objects = append(objects, ObjectInfo{
        	Bucket:			bucket,
    		Name:			name,
    		Size:       	size,
    		Queued:         time.Unix(queued, 0),
    		Uploaded: 		time.Unix(0, 0),
    		PurgeAfter:		purge_after,
    		CachedFetches:	cached_fetches,
    		SiaFetches:		sia_fetches,
    		LastFetch:  	time.Unix(last_fetch, 0),
    	})
    }

    rows.Close()

	return objects, nil
}

func (b *SiaBridge) insertBucket(bucket string) error {
	stmt, err := g_db.Prepare("INSERT INTO buckets(name, created) values(?,?)")
    if err != nil {
    	return err
    }

    _, err = stmt.Exec(bucket, time.Now().Unix())
    if err != nil {
    	return err
    }

    return nil
}

func (b *SiaBridge) insertObject(bucket string, objectName string, size int64, queued int64, uploaded int64, purge_after int64) error {
	stmt, err := g_db.Prepare("INSERT INTO objects(bucket, name, size, queued, uploaded, purge_after, cached_fetches, sia_fetches, last_fetch) values(?,?,?,?,?,?,?,?,?)")
    if err != nil {
    	return err
    }

    _, err = stmt.Exec(bucket,
						objectName,
						size,
						queued,
						uploaded,
						purge_after,
						0,
						0,
						-1)
    if err != nil {
    	return err
    }

    return nil
}
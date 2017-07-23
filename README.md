# SiaBridge
A bridge to using Sia Decentralized Private Cloud Storage in any Go application!

### What is Sia?
Sia is a blockchain-based decentralized storage service with built-in privacy and redundancy that costs up to 10x LESS than Amazon S3 and most other cloud providers! See [sia.tech](https://sia.tech) to learn how awesome Sia truly is.

### What does SiaBridge do?
SiaBridge is a package that can be imported into any Go application. It provides a simple API for creating and deleting buckets (folders) and uploading/downloading files to/from those buckets, all of which is stored privately, redundantly, and inexpensively on the Sia network.

Due to the decentralized nature of the network, there is a bit of latency involved when uploading and downloading. SiaBridge aims to eliminate most of the perceived latency by transparently providing a caching layer between the end-user API and the Sia network. From the end-user's perspective, uploads are instantaneous and frequently-accessed files can be downloaded quickly and with no noticeable latency. Different caching attributes can be assigned to different objects to optimize the bridge for any storage scenario.

SiaBridge has the added benefit of reducing download bandwidth from the Sia network, which saves additional money!

### Sounds great! How do I get started?
Look at the main.go source file for an example of how to use the SiaBridge package in your application to store and retrieve data.

#### Import the SiaBridge Package
First, you'll need to make sure your application imports the SiaBridge package at the top of your source file.
```go
import (
    "github.com/dvstate/siabridge/bridge"
)
```

#### Initialize the SiaBridge API
Next, you'll need to initialize the SiaBridge API object.
```go
siab := &bridge.SiaBridge{"127.0.0.1:9980", ".sia_cache", "siabridge.db"}           
```
The first parameter is the address and port of the Sia daemon, the second parameter is the name of the local cache directory, and the final parameter is the name of the local database file.

#### Starting the SiaBridge
Before any other SiaBridge API calls are made, the SiaBridge has to be started.
```go
err := siab.Start()
```
The error returned from Start should equal nil if successful.

#### Creating a Bucket
To create a bucket (folder) to store files, simply use the CreateBucket method.
```go
err := siab.CreateBucket("MyBucket")
```
#### Listing Buckets
To list all existing buckets, use the ListBuckets method.
```go
buckets, err := siab.ListBuckets()
if err != nil {
  return err
}

for _, bucket := range buckets {
  fmt.Printf("  %s (Created: %s)\n", bucket.Name, bucket.Created)
}
```
#### Deleting a Bucket
To delete a bucket, simply use the DeleteBucket method.
```go
err = g_siab.DeleteBucket("MyBucket")
```

#### Storing an Object
To store an object on the Sia network, use either PutObjectFromReader or PutObjectFromFile.
```go
err = siab.PutObjectFromFile("LocalFile.txt", "MyBucket", "RemoteFile.txt", 24*60*60)
```
The above code would take LocalFile.txt and upload it to Sia as MyBucket/RemoteFile.txt. The final parameter is the maximum number of seconds since last fetch to keep the file cached. In the above example, the object will be removed from the local cache after 24 hours since it was last requested. After an object is removed from the local cache, the next time the API requests it, a download will be triggered from the Sia network.
```go
file := "LocalFile.txt"

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
err = siab.PutObjectFromReader(data, "MyBucket", "RemoteFile.txt", size, 24*60*60)
```
The above code example does the same thing, but demonstrates the use of the PutObjectFromReader method.

#### Fetching an Object
To download an object from the Sia network (or the cache), use the GetObject method.
```go
writer, err = os.Create("DownloadedFile.txt")
if err != nil {
    return err
}

err = siab.GetObject("MyBucket", "RemoteFile.txt", writer)
```
The above code will download "MyBucket/RemoteFile.txt" from either the cache or Sia network and store it in the local file "DownloadedFile.txt".

#### Deleting an Object
To delete an object, use the DeleteObject method.
```go
err := siab.DeleteObject("MyBucket", "RemoteFile.txt")
```
The above code will delete "RemoteFile.txt" from "MyBucket".

#### Listing Objects in a Bucket
To get a list of all objects stored in a bucket, use the ListObjects method.
```go
objects, err := siab.ListObjects("MyBucket")
if err != nil {
    return err
}

for _, obj := range objects {
    // obj.Bucket - stores name of bucket
    // obj.Name - stores name of object   
    // obj.Queued - stores time.Time of when object was originally "Put"
    // obj.Uploaded - stores time.Time of when object completely uploaded to Sia
    // obj.Size - stores size of file in bytes
    // obj.PurgeAfter - stores number of seconds to live in cache without fetch
    // obj.CachedFetches - stores total number of times object served from cache
    // obj.SiaFetches - stores total number of times object served from Sia
    // obj.LastFetch - stores time.Time of when object was last fetched
}
```
As you can see, there's a lot of useful information maintained for each object, such as how many times the object is served from cache versus from Sia, etc.

#### Getting Information for Specific Object
To get information for a specific object, use the GetObjectInfo method.
```go
objInfo, err := siab.GetObjectInfo("MyBucket", "RemoteFile.txt")
```
The above example would obtain the info for just "MyBucket/RemoteFile.txt".

#### Stopping the SiaBridge
Before exiting your application, the SiaBridge should be stopped.
```go
siab.Stop()
```

### Prerequisites
To use SiaBridge, you must have an up-to-date copy of the Sia daemon running. The Sia daemon must be fully synchronized with the Sia network. You must have active rental contracts that you've acquired using the Sia-UI or siac command line utility. To purchase inexpensive rental contracts, you have to possess some Siacoin in your wallet. To obtain Siacoin, you will need to purchase some on an exchange such as Bittrex using bitcoin. To obtain bitcoin, you'll need to use a service such as Coinbase to buy bitcoin using a bank account or credit card. If you need help, there are many friendly people active on [Sia's Slack](http://slackin.sia.tech).

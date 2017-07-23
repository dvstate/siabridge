package main

import (
	"fmt"
	"time"
	"os"
	"path/filepath"
	"github.com/dvstate/siabridge/bridge"
)

var g_siab *bridge.SiaBridge

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func listBuckets() {
	fmt.Println("Listing buckets:")

	buckets, err := g_siab.ListBuckets()
	checkError(err)

	for _, bucket := range buckets {
		fmt.Printf("  %s (Created: %s)\n", bucket.Name, bucket.Created)
	}
}

func listObjects(bucket string) {
	fmt.Printf("Listing objects in bucket: %s\n", bucket)

	objects, err := g_siab.ListObjects(bucket)
	checkError(err)

	for _, obj := range objects {
		fmt.Printf("  %s (Queued: %s) (Uploaded: %s)\n", 
			obj.Name, 
			obj.Queued,
			obj.Uploaded)
	}
}

func createTestFile(path, text string) error {
	f, err := os.Create(path)
	if err != nil {
	      return err
	}
	defer f.Close()

	_, err = f.WriteString(text)
	if err != nil {
	      return err
	}

	f.Sync()

	return nil
}

func main() {
	g_siab := &bridge.SiaBridge{"127.0.0.1:9980",
								".sia_cache",
	                            "siabridge.db"}

	err := g_siab.Start()
	checkError(err)
	fmt.Println("Started Sia Bridge")

	fmt.Println("Creating buckets:")
	idx := 1
	for idx <= 5 {
		err = g_siab.CreateBucket(fmt.Sprintf("TestBucket%d",idx))
		checkError(err)
		fmt.Printf("  TestBucket%d\n",idx)

		idx += 1
	}

	listBuckets()

	fmt.Println("Deleting bucket: TestBucket5")
	err = g_siab.DeleteBucket("TestBucket5")
	checkError(err)

	listBuckets()

	fmt.Println("Creating TestFile1.txt");
	err = createTestFile("TestFile1.txt", "This is test file 1\n")
	checkError(err)

	fmt.Println("Uploading TestFile1.txt to TestBucket1")
	// Purge from local cache if not downloaded any in 24 hours
	err = g_siab.PutObjectFromFile("./TestFile1.txt", "TestBucket1", "TestFile1.txt", 24*60*60)
	if err != nil {
		fmt.Println(err)
	}

	listObjects("TestBucket1")

	fmt.Println("FROM CACHE: Downloading TestFile1.txt from TestBucket1 as TestFile1-Cache.txt")
	writer, err := os.Create("TestFile1-Cache.txt")
	checkError(err)

	err = g_siab.GetObject("TestBucket1", "TestFile1.txt", writer)
	writer.Sync()
	writer.Close()
	checkError(err)
	fmt.Println("Download complete")

	objInfo, err := g_siab.GetObjectInfo("TestBucket1", "TestFile1.txt")
	checkError(err)
	for objInfo.Uploaded == time.Unix(0,0) {
		fmt.Println("Waiting for TestFile1.txt to upload to Sia...")
		time.Sleep(time.Millisecond * 5000)
		objInfo, err = g_siab.GetObjectInfo("TestBucket1", "TestFile1.txt")
		checkError(err)
	}

	fmt.Println("Deleting TestFile1.txt from cache")
	err = os.Remove(filepath.Join(".sia_cache","TestBucket1","TestFile1.txt"))
	checkError(err)

	fmt.Println("FROM SIA: Downloading TestFile1.txt from TestBucket1 as TestFile1-Sia.txt")
	writer, err = os.Create("TestFile1-Sia.txt")
	checkError(err)

	err = g_siab.GetObject("TestBucket1", "TestFile1.txt", writer)
	checkError(err)
	fmt.Println("Download complete")

	g_siab.Stop()

}
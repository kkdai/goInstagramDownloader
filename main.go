package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/user"
	"strings"
	"sync"

	"github.com/gedex/go-instagram/instagram"
)

var instaName = flag.String("n", "", "Instangram user name such as: 'kingjames'")
var numOfWorkersPtr = flag.Int("c", 2, "the number of concurrent rename workers. default = 2")

var m sync.Mutex
var FileIndex int = 0
var client *instagram.Client

func GetFileIndex() (ret int) {
	m.Lock()
	ret = FileIndex
	FileIndex = FileIndex + 1
	m.Unlock()
	return ret
}

var ClientID string

func init() {
	ClientID = os.Getenv("InstagramID")
	if ClientID == "" {
		log.Fatalln("Please set 'export InstagramID=xxxxx' as your environment variables")

	}
}

func DownloadWorker(destDir string, linkChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for target := range linkChan {
		var imageType string
		if strings.Contains(target, ".png") {
			imageType = ".png"
		} else {
			imageType = ".jpg"
		}

		resp, err := http.Get(target)
		if err != nil {
			log.Println("Http.Get\nerror: " + err.Error() + "\ntarget: " + target)
			continue
		}
		defer resp.Body.Close()

		m, _, err := image.Decode(resp.Body)
		if err != nil {
			log.Println("image.Decode\nerror: " + err.Error() + "\ntarget: " + target)
			continue
		}

		// Ignore small images
		bounds := m.Bounds()
		if bounds.Size().X > 300 && bounds.Size().Y > 300 {
			imgInfo := fmt.Sprintf("pic%04d", GetFileIndex())
			out, err := os.Create(destDir + "/" + imgInfo + imageType)
			if err != nil {
				log.Println("os.Create\nerror: %s", err)
				continue
			}
			defer out.Close()
			if imageType == ".png" {
				png.Encode(out, m)
			} else {
				jpeg.Encode(out, m, nil)
			}

			if FileIndex%30 == 0 {
				fmt.Println(FileIndex, " photos downloaded.")
			}
		}
	}
}

func FindPhotos(ownerName string, albumName string, userId string, baseDir string) {
	totalPhotoNumber := 1
	var mediaList []instagram.Media
	var next *instagram.ResponsePagination
	var optParam *instagram.Parameters
	var err error

	//Create folder
	dir := fmt.Sprintf("%v/%v", baseDir, ownerName)
	os.MkdirAll(dir, 0755)
	linkChan := make(chan string)
	//Create download worker
	wg := new(sync.WaitGroup)
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go DownloadWorker(dir, linkChan, wg)
	}

	for true {
		maxId := ""
		if next != nil {
			maxId = next.NextMaxID
		}

		optParam = &instagram.Parameters{Count: 10, MaxID: maxId}
		mediaList, next, err = client.Users.RecentMedia(userId, optParam)
		if err != nil {
			log.Println("err:", err)
			break
		}

		for _, media := range mediaList {
			totalPhotoNumber = totalPhotoNumber + 1
			linkChan <- media.Images.StandardResolution.URL
		}

		if len(mediaList) == 0 || next.NextMaxID == "" {
			break
		}
	}
}

func main() {
	flag.Parse()
	var inputUser string
	if *instaName == "" {
		log.Fatalln("You need to input -n=Name.")
	}
	inputUser = *instaName

	//Get system user folder
	usr, _ := user.Current()
	baseDir := fmt.Sprintf("%v/Pictures/goInstagram", usr.HomeDir)

	//Get User info
	client = instagram.NewClient(nil)
	client.ClientID = ClientID

	var userId string
	searchUsers, _, err := client.Users.Search(inputUser, nil)
	for _, user := range searchUsers {
		if user.Username == inputUser {
			userId = user.ID
		}
	}

	if userId == "" {
		log.Fatalln("Can not address user name: ", inputUser, err)
	}

	userFolderName := fmt.Sprintf("[%s]%s", userId, inputUser)
	fmt.Println("Starting download [", userId, "]", inputUser)
	FindPhotos(userFolderName, inputUser, userId, baseDir)
}

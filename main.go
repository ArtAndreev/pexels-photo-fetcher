package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type (
	Response struct {
		TotalResults int     `json:"total_results"`
		Page         int     `json:"page"`
		PerPage      int     `json:"per_page"`
		Photos       []Photo `json:"photos"`
		NextPage     string  `json:"next_page"`
	}

	Photo struct {
		ID              int    `json:"id"`
		Width           int    `json:"width"`
		Height          int    `json:"height"`
		URL             string `json:"url"`
		Photographer    string `json:"photographer"`
		PhotographerURL string `json:"photographer_url"`
		PhotographerID  int    `json:"photographer_id"`
		Src             Src    `json:"src"`
		Liked           bool   `json:"liked"`
	}

	Src struct {
		Original  string `json:"original"`
		Large2x   string `json:"large2x"`
		Large     string `json:"large"`
		Medium    string `json:"medium"`
		Small     string `json:"small"`
		Portrait  string `json:"portrait"`
		Landscape string `json:"landscape"`
		Tiny      string `json:"tiny"`
	}
)

func main() {
	key := flag.String("key", "", "authorization key")
	dst := flag.String("dst", "", "folder to save photos")
	query := flag.String("query", "people", "query for search")

	flag.Parse()

	if *key == "" {
		log.Fatal("no key provided")
	}

	if *dst == "" {
		const defDst = "output"

		log.Printf("using default destination %q at work directory", defDst)

		*dst = defDst
	}

	if err := os.Mkdir(*dst, 0755); err != nil {
		if !os.IsExist(err) {
			log.Fatalf("cannot create destination directory: %s", err)
		}
	}

	nextPage, err := compileFirstURL(*query)
	if err != nil {
		log.Fatalf("compile first url: %s", err)
	}

	totalCount := 0

	var client http.Client

	for {
		resp, err := sendRequest(&client, nextPage, *key)
		if err != nil {
			log.Fatalf("send request: %s, url: %s", err, nextPage)
		}

		totalCount += len(resp.Photos)

		for _, p := range resp.Photos {
			photoURL := p.Src.Large2x
			if err = processPhoto(&client, photoURL, *dst); err != nil {
				log.Fatalf("failed to process photo %s: %s", photoURL, err)
			}
		}

		log.Printf("processed: %d", totalCount)

		if nextPage = resp.NextPage; nextPage == "" {
			break
		}
	}

	log.Printf("done, total count is %d", totalCount)
}

func compileFirstURL(query string) (string, error) {
	const firstURL = "https://api.pexels.com/v1/search?per_page=80&page=1"

	u, err := url.Parse(firstURL)
	if err != nil {
		return "", fmt.Errorf("parse first url %s: %w", firstURL, err)
	}

	q := u.Query()
	q.Set("query", query)

	u.RawQuery = q.Encode()

	return u.String(), nil
}

func sendRequest(client *http.Client, url string, key string) (Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Response{}, fmt.Errorf("create new request: %w", err)
	}

	req.Header.Add("Authorization", key)

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("cannot send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("got non-200 response")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("cannot read response: %w", err)
	}

	var response Response

	if err = json.Unmarshal(body, &response); err != nil {
		log.Fatalf("cannot unmarshal json: %s, body: %q", err, body)
	}

	return response, nil
}

func processPhoto(client *http.Client, photoURL, dst string) error {
	img, err := downloadImage(client, photoURL)
	if err != nil {
		return fmt.Errorf("download image: %w", err)
	}
	defer img.Close()

	// Cut off query args
	if idx := strings.LastIndex(photoURL, "?"); idx != -1 {
		photoURL = photoURL[:idx]
	}

	name := path.Base(photoURL)
	fullPath := filepath.Join(dst, name)

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", fullPath, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, img); err != nil {
		return fmt.Errorf("write file %s: %w", fullPath, err)
	}

	return nil
}

func downloadImage(client *http.Client, url string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create new request: %w", err)
	}

	resp, err := client.Do(req) //nolint:bodyclose // should be closed outside
	if err != nil {
		return nil, fmt.Errorf("cannot send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got non-200 response")
	}

	return resp.Body, nil
}

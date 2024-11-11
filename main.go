package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}
	serverURL := "http://localhost:8080"
	command := os.Args[1]
	switch command {
	case "upload":
		uploadCommand(serverURL)
	case "shorten":
		shortenCommand(serverURL)
	case "stats":
		statsCommand(serverURL)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  client upload [-long] <file_path>")
	fmt.Println("  client shorten [-long] <url>")
	fmt.Println("  client stats")
}

func uploadCommand(serverURL string) {
	uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)
	longFlag := uploadCmd.Bool("long", false, "Keep the file for a month")
	uploadCmd.Parse(os.Args[2:])
	if uploadCmd.NArg() < 1 {
		fmt.Println("Usage: client upload [-long] <file_path>")
		return
	}
	filePath := uploadCmd.Arg(0)
	err := uploadFile(serverURL+"/upload", filePath, *longFlag)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func shortenCommand(serverURL string) {
	shortenCmd := flag.NewFlagSet("shorten", flag.ExitOnError)
	longFlag := shortenCmd.Bool("long", false, "Keep the URL for a month")
	shortenCmd.Parse(os.Args[2:])
	if shortenCmd.NArg() < 1 {
		fmt.Println("Usage: client shorten [-long] <url>")
		return
	}
	url := shortenCmd.Arg(0)
	err := shortenURL(serverURL+"/shorten", url, *longFlag)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func statsCommand(serverURL string) {
	resp, err := http.Get(serverURL + "/stats?format=json")
	if err != nil {
		fmt.Printf("Error fetching stats: %v\n", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
}

func uploadFile(url, filePath string, long bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", filepath.Base(file.Name()))
	io.Copy(part, file)
	if long {
		writer.WriteField("long", "true")
	}
	writer.Close()
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	return nil
}

func shortenURL(url, originalURL string, long bool) error {
	data := map[string]interface{}{
		"url":  originalURL,
		"long": long,
	}
	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	return nil
}

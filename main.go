package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/progressbar/v3"
)

type Response struct {
	Message  string `json:"message"`
	ShortURL string `json:"short_url"`
	Filename string `json:"filename,omitempty"`
	ID       string `json:"id,omitempty"`
	Url      string `json:"url,omitempty"`
}

func main() {
	serverURL := "https://keiran.cc"
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	// Try to guess the command if it's not recognized
	if !isKnownCommand(command) {
		command = guessCommand(command)
		if command == "" {
			fmt.Println("Could not determine what you want to do.")
			printUsage()
			return
		}
	}

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

func isKnownCommand(cmd string) bool {
	commands := []string{"upload", "shorten", "stats"}
	for _, c := range commands {
		if c == cmd {
			return true
		}
	}
	return false
}

func guessCommand(arg string) string {
	// Check if it's a file
	if _, err := os.Stat(arg); err == nil {
		return "upload"
	}

	// Check if it looks like a URL
	if _, err := url.ParseRequestURI(arg); err == nil {
		return "shorten"
	}

	return ""
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  client upload [-long] <file_path>")
	fmt.Println("  client shorten [-long] <url>")
	fmt.Println("  client stats")
	fmt.Println("\nYou can also directly provide a file or URL:")
	fmt.Println("  client <file_path>    (uploads the file)")
	fmt.Println("  client <url>          (shortens the URL)")
}

func uploadCommand(serverURL string) {
	uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)
	longFlag := uploadCmd.Bool("long", false, "Keep the file for a month")

	args := os.Args[1:]
	if args[0] == "upload" {
		args = args[1:] // Remove "upload" if it's present
	}
	uploadCmd.Parse(args)

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

	args := os.Args[1:]
	if args[0] == "shorten" {
		args = args[1:] // Remove "shorten" if it's present
	}
	shortenCmd.Parse(args)

	if shortenCmd.NArg() < 1 {
		fmt.Println("Usage: client shorten [-long] <url>")
		return
	}

	urlToShorten := shortenCmd.Arg(0)
	err := shortenURL(serverURL+"/shorten", urlToShorten, *longFlag)
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

	var stats struct {
		Files int `json:"files"`
		URLs  int `json:"urls"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Printf("Error parsing stats: %v\n", err)
		return
	}

	fmt.Printf("📊 Statistics:\n")
	fmt.Printf("Files stored: %d\n", stats.Files)
	fmt.Printf("URLs shortened: %d\n", stats.URLs)
}

func uploadFile(url, filePath string, long bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("couldn't open file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("couldn't get file info: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if long {
		if err := writer.WriteField("long", "true"); err != nil {
			return fmt.Errorf("error writing long field: %v", err)
		}
	}

	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return fmt.Errorf("couldn't create form file: %v", err)
	}

	bar := progressbar.NewOptions64(
		fileInfo.Size(),
		progressbar.OptionSetDescription("📤 Uploading"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	_, err = io.Copy(io.MultiWriter(part, bar), file)
	if err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("error closing multipart writer: %v", err)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned error %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error parsing response: %v", err)
	}

	fmt.Printf("\n✨ Success! Your file has been uploaded\n")
	fmt.Printf("📎 URL: %s\n", response.Url)
	if response.ID != "" {
		fmt.Printf("🔑 ID: %s\n", response.ID)
	}

	return nil
}

func shortenURL(url, originalURL string, long bool) error {
	data := map[string]interface{}{
		"url":  originalURL,
		"long": long,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error encoding request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error parsing response: %v", err)
	}

	fmt.Printf("✨ Success! Your URL has been shortened\n")
	fmt.Printf("🔗 Short URL: %s\n", response.ShortURL)

	return nil
}

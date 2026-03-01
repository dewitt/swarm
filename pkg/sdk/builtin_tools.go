package sdk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

type ListFilesArgs struct {
	Dir       string `json:"dir"`
	Recursive bool   `json:"recursive,omitempty"`
}

type ListFilesResult struct {
	Files []string `json:"files"`
	Error string   `json:"error,omitempty"`
}

func listLocalFiles(ctx tool.Context, args ListFilesArgs) (ListFilesResult, error) {
	if args.Dir == "" {
		args.Dir = "."
	}
	var files []string

	if args.Recursive {
		err := filepath.Walk(args.Dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors (like permission denied)
			}
			// Skip hidden directories like .git
			if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if path != args.Dir {
				name := strings.TrimPrefix(path, args.Dir+"/")
				if info.IsDir() {
					name += "/"
				}
				files = append(files, name)
			}
			return nil
		})
		if err != nil {
			return ListFilesResult{Error: err.Error()}, nil
		}
	} else {
		entries, err := os.ReadDir(args.Dir)
		if err != nil {
			return ListFilesResult{Error: err.Error()}, nil
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			files = append(files, name)
		}
	}

	// Limit to prevent context window explosion
	if len(files) > 1000 {
		files = append(files[:1000], fmt.Sprintf("... and %d more. Use grep_search for specific queries.", len(files)-1000))
	}

	return ListFilesResult{Files: files}, nil
}

type ReadFileArgs struct {
	Path string `json:"path"`
}
type ReadFileResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func readLocalFile(ctx tool.Context, args ReadFileArgs) (ReadFileResult, error) {
	b, err := os.ReadFile(args.Path)
	if err != nil {
		return ReadFileResult{Error: err.Error()}, nil
	}
	return ReadFileResult{Content: string(b)}, nil
}

type GrepArgs struct {
	Pattern string `json:"pattern"`
	Dir     string `json:"dir"`
}
type GrepResult struct {
	Matches []string `json:"matches"`
	Error   string   `json:"error,omitempty"`
}

func grepSearch(ctx tool.Context, args GrepArgs) (GrepResult, error) {
	if args.Dir == "" {
		args.Dir = "."
	}
	cmd := exec.Command("grep", "-r", "-l", args.Pattern, args.Dir)
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return GrepResult{Matches: []string{}}, nil
		}
		return GrepResult{Error: err.Error()}, nil
	}
	matches := strings.Split(strings.TrimSpace(string(out)), "\n")
	return GrepResult{Matches: matches}, nil
}

type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type WriteFileResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func writeLocalFile(ctx tool.Context, args WriteFileArgs) (WriteFileResult, error) {
	dir := filepath.Dir(args.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return WriteFileResult{Success: false, Error: err.Error()}, nil
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
		return WriteFileResult{Success: false, Error: err.Error()}, nil
	}
	return WriteFileResult{Success: true}, nil
}

type WebFetchArgs struct {
	URL string `json:"url"`
}
type WebFetchResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func webFetch(ctx tool.Context, args WebFetchArgs) (WebFetchResult, error) {
	resp, err := http.Get(args.URL)
	if err != nil {
		return WebFetchResult{Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return WebFetchResult{Error: err.Error()}, nil
	}
	return WebFetchResult{Content: string(b)}, nil
}

type GoogleSearchArgs struct {
	Query string `json:"query"`
}
type GoogleSearchResult struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func googleSearchFunc(ctx tool.Context, args GoogleSearchArgs) (GoogleSearchResult, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return GoogleSearchResult{Error: "GOOGLE_API_KEY is not set"}, nil
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return GoogleSearchResult{Error: err.Error()}, nil
	}
	resp, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash", genai.Text(args.Query), &genai.GenerateContentConfig{Tools: []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}}})
	if err != nil {
		return GoogleSearchResult{Error: err.Error()}, nil
	}
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		return GoogleSearchResult{Response: resp.Candidates[0].Content.Parts[0].Text}, nil
	}
	return GoogleSearchResult{Response: "No results found"}, nil
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ---------- MODELS ----------

type Note struct {
	ID        int    `json:"id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type Report struct {
	ID        int    `json:"id"`
	Date      string `json:"date"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

type JournalStore struct {
	Notes   []Note   `json:"notes"`
	Reports []Report `json:"reports"`
	NextID  int      `json:"next_id"`
}

type ToolRequest struct {
	Tool    string `json:"tool"`
	Text    string `json:"text"`
	Summary string `json:"summary"`
}

type ToolResponse struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ---------- OPENROUTER ----------

type ORRequest struct {
	Model    string      `json:"model"`
	Messages []ORMessage `json:"messages"`
}

type ORMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ORResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const storeFile = "journal.json"

// ---------- STORE ----------

func loadStore() JournalStore {
	data, err := os.ReadFile(storeFile)
	if err != nil {
		return JournalStore{NextID: 1, Notes: []Note{}, Reports: []Report{}}
	}

	var store JournalStore
	json.Unmarshal(data, &store)

	if store.Notes == nil {
		store.Notes = []Note{}
	}
	if store.Reports == nil {
		store.Reports = []Report{}
	}

	return store
}

func saveStore(store JournalStore) {
	data, _ := json.MarshalIndent(store, "", "  ")
	os.WriteFile(storeFile, data, 0644)
}

// ---------- AI REPORT ----------

func generateReportWithAI(notes string) (string, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	today := time.Now().Format("02 January 2006")

	prompt := fmt.Sprintf(`You are a daily learning report generator.

NOTES:
%s

Generate:
- Work done
- Concepts learned
- Tools used
- Summary

Date: %s`, notes, today)

	body, _ := json.Marshal(ORRequest{
		Model:    "mistralai/mistral-small-24b-instruct-2501",
		Messages: []ORMessage{{Role: "user", Content: prompt}},
	})

	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var result ORResponse
	json.Unmarshal(raw, &result)

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no AI response")
	}

	return result.Choices[0].Message.Content, nil
}

// ---------- HANDLERS ----------

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "📔 Journal MCP Server is running!")
}

func toolHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req ToolRequest
	json.NewDecoder(r.Body).Decode(&req)

	store := loadStore()

	switch req.Tool {

	case "save_note":
		if req.Text == "" {
			json.NewEncoder(w).Encode(ToolResponse{Error: "text required"})
			return
		}

		note := Note{
			ID:        store.NextID,
			Text:      req.Text,
			CreatedAt: time.Now().Format("2006-01-02 15:04"),
		}

		store.Notes = append(store.Notes, note)
		store.NextID++
		saveStore(store)

		json.NewEncoder(w).Encode(ToolResponse{
			Result: fmt.Sprintf("✅ Note #%d saved", note.ID),
		})

	case "list_notes":
		if len(store.Notes) == 0 {
			json.NewEncoder(w).Encode(ToolResponse{Result: "No notes saved yet!"})
			return
		}

		result := "📝 Your Notes:\n"
		for _, n := range store.Notes {
			result += fmt.Sprintf("\n[#%d] %s | saved: %s", n.ID, n.Text, n.CreatedAt)
		}

		json.NewEncoder(w).Encode(ToolResponse{Result: result})

	case "generate_report":
		if len(store.Notes) == 0 {
			json.NewEncoder(w).Encode(ToolResponse{Error: "No notes found!"})
			return
		}

		var all string
		for _, n := range store.Notes {
			all += fmt.Sprintf("[%d] %s\n", n.ID, n.Text)
		}

		report, err := generateReportWithAI(all)
		if err != nil {
			json.NewEncoder(w).Encode(ToolResponse{Error: err.Error()})
			return
		}

		r := Report{
			ID:        store.NextID,
			Date:      time.Now().Format("2006-01-02"),
			Summary:   report,
			CreatedAt: time.Now().Format("2006-01-02 15:04"),
		}

		store.Reports = append(store.Reports, r)
		store.NextID++
		saveStore(store)

		json.NewEncoder(w).Encode(ToolResponse{Result: report})

	case "list_reports":
		if len(store.Reports) == 0 {
			json.NewEncoder(w).Encode(ToolResponse{Result: "No reports saved yet!"})
			return
		}

		result := "📋 Your Reports:\n"
		for _, r := range store.Reports {
			result += fmt.Sprintf("\n[#%d] Date: %s\n%s\n", r.ID, r.Date, r.Summary)
		}

		json.NewEncoder(w).Encode(ToolResponse{Result: result})

	default:
		json.NewEncoder(w).Encode(ToolResponse{Error: "Unknown tool"})
	}
}

// ---------- MAIN ----------

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/tool", toolHandler)
	http.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "journal_app.html")
	})

	fmt.Println("✅ Journal MCP Server running at http://localhost:8082")
	fmt.Println("📌 Endpoint: POST http://localhost:8082/tool")
	fmt.Println("🌐 App:      http://localhost:8082/app")
	fmt.Println("🛠️  Tools:   save_note | list_notes | generate_report | list_reports")

	http.ListenAndServe(":8082", nil)
}
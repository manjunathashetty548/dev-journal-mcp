package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// ---------- MCP TYPES ----------

type MCPRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type MCPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ---------- RESPONSE HELPERS ----------

func send(id interface{}, result interface{}) {
	resp := MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendErr(id interface{}, msg string) {
	resp := MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    -1,
			"message": msg,
		},
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

// ---------- BACKEND CALL ----------

func callBackend(payload map[string]interface{}) (string, error) {
	b, _ := json.Marshal(payload)

	resp, err := http.Post("http://localhost:8082/tool", "application/json", bytes.NewBuffer(b))
	if err != nil {
		return "", fmt.Errorf("backend not reachable: %w", err)
	}
	defer resp.Body.Close()

	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)

	if out["error"] != nil {
		return "", fmt.Errorf("%v", out["error"])
	}

	return fmt.Sprintf("%v", out["result"]), nil
}

// ---------- MAIN MCP LOOP ----------

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		switch req.Method {

		// ---------- INIT ----------
		case "initialize":
			send(req.ID, map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "journal-mcp",
					"version": "1.0.0",
				},
			})

		case "notifications/initialized":
			// no response needed

		// ---------- TOOL LIST ----------
		case "tools/list":
			send(req.ID, map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "save_note",
						"description": "Save a note",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text": map[string]string{
									"type":        "string",
									"description": "The note text",
								},
							},
							"required": []string{"text"},
						},
					},
					{
						"name":        "list_notes",
						"description": "List all notes",
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
					{
						"name":        "generate_report",
						"description": "Generate report from notes",
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
					{
						"name":        "list_reports",
						"description": "List reports",
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
			})

		// ---------- TOOL CALL ----------
		case "tools/call":
			var p ToolCallParams
			json.Unmarshal(req.Params, &p)

			payload := map[string]interface{}{
				"tool": p.Name,
			}

			if text, ok := p.Arguments["text"]; ok {
				payload["text"] = text
			}

			res, err := callBackend(payload)

			if err != nil {
				sendErr(req.ID, err.Error())
				continue
			}

			send(req.ID, map[string]interface{}{
				"content": []map[string]string{
					{
						"type": "text",
						"text": res,
					},
				},
			})

		// ---------- UNKNOWN ----------
		default:
			if req.ID != nil {
				sendErr(req.ID, "unknown method: "+req.Method)
			}
		}
	}
}
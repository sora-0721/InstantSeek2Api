package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekRequest struct {
	Message        string `json:"message"`
	ConversationId any    `json:"conversationId"`
}

type DeepSeekResponse struct {
	Response       string `json:"response"`
	ConversationId string `json:"conversation_id"`
}

type OpenAIResponse struct {
	Id      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type StreamChoice struct {
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

type OpenAIStreamResponse struct {
	Id      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Index        int     `json:"index"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	authToken := os.Getenv("AUTH_TOKEN")
	if authToken != "" {
		providedToken := r.Header.Get("Authorization")
		if providedToken != "Bearer "+authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if r.URL.Path != "/v1/chat/completions" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "InstantSeek2Api Service Running...",
			"message": "MoLoveSze...",
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var openAIReq OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&openAIReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if openAIReq.Model != "deepseek-chat" {
		http.Error(w, "Only deepseek-chat model is supported", http.StatusBadRequest)
		return
	}

	lastMessage := openAIReq.Messages[len(openAIReq.Messages)-1].Content

	deepSeekReq := DeepSeekRequest{
		Message:        lastMessage,
		ConversationId: nil,
	}

	reqBody, err := json.Marshal(deepSeekReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://instantseek.org/api/chat", bytes.NewBuffer(reqBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	req.Header.Set("sec-ch-ua-platform", "Windows")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0")
	req.Header.Set("sec-ch-ua", "\"Not(A:Brand\";v=\"99\", \"Microsoft Edge\";v=\"133\", \"Chromium\";v=\"133\"")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("host", "instantseek.org")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var deepSeekResp DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stream := openAIReq.Stream || r.Header.Get("Accept") == "text/event-stream"

	openAIResp := OpenAIResponse{
		Id:      "chatcmpl-" + deepSeekResp.ConversationId,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "deepseek-chat",
		Choices: []Choice{
			{
				Message: Message{
					Role:    "assistant",
					Content: deepSeekResp.Response,
				},
				FinishReason: "stop",
				Index:        0,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")

	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		roleResp := OpenAIStreamResponse{
			Id:      openAIResp.Id,
			Object:  "chat.completion.chunk",
			Created: openAIResp.Created,
			Model:   openAIResp.Model,
			Choices: []StreamChoice{
				{
					Index: 0,
					Delta: Delta{
						Role: "assistant",
					},
				},
			},
		}
		fmt.Fprintf(w, "data: %s\n\n", mustMarshal(roleResp))
		contentResp := OpenAIStreamResponse{
			Id:      openAIResp.Id,
			Object:  "chat.completion.chunk",
			Created: openAIResp.Created,
			Model:   openAIResp.Model,
			Choices: []StreamChoice{
				{
					Index: 0,
					Delta: Delta{
						Content: deepSeekResp.Response,
					},
				},
			},
		}
		fmt.Fprintf(w, "data: %s\n\n", mustMarshal(contentResp))
		finishResp := OpenAIStreamResponse{
			Id:      openAIResp.Id,
			Object:  "chat.completion.chunk",
			Created: openAIResp.Created,
			Model:   openAIResp.Model,
			Choices: []StreamChoice{
				{
					Index:        0,
					Delta:        Delta{},
					FinishReason: "stop",
				},
			},
		}
		fmt.Fprintf(w, "data: %s\n\n", mustMarshal(finishResp))
		fmt.Fprintf(w, "data: [DONE]\n\n")
	} else {
		json.NewEncoder(w).Encode(openAIResp)
	}
}

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

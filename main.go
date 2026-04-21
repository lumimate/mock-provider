package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// --- OpenAI-compatible types ---

type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
}

type ChatCompletionChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
	Usage   *Usage                       `json:"usage,omitempty"`
}

type ChatCompletionStreamChoice struct {
	Index        int          `json:"index"`
	Delta        MessageDelta `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

type MessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

const mockReply = "This is a mock response for latency testing."

func main() {
	port := flag.Int("port", 8199, "listen port")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handleChatCompletions)
	mux.HandleFunc("/v1/embeddings", handleEmbeddings)
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("mock-provider listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Stream {
		handleStreamChat(w, req)
		return
	}

	now := time.Now().Unix()
	resp := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-mock-%d", now),
		Object:  "chat.completion",
		Created: now,
		Model:   req.Model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: mockReply,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 9,
			TotalTokens:      19,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStreamChat(w http.ResponseWriter, req ChatCompletionRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	now := time.Now().Unix()
	id := fmt.Sprintf("chatcmpl-mock-%d", now)

	// First chunk: role
	sendSSE(w, flusher, ChatCompletionStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: now,
		Model:   req.Model,
		Choices: []ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: MessageDelta{Role: "assistant"},
			},
		},
	})

	// Content chunks: split by words for realistic streaming
	words := strings.Fields(mockReply)
	for i, word := range words {
		content := word
		if i < len(words)-1 {
			content += " "
		}
		sendSSE(w, flusher, ChatCompletionStreamResponse{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: now,
			Model:   req.Model,
			Choices: []ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: MessageDelta{Content: content},
				},
			},
		})
	}

	// Final chunk: finish_reason + usage
	stop := "stop"
	sendSSE(w, flusher, ChatCompletionStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: now,
		Model:   req.Model,
		Choices: []ChatCompletionStreamChoice{
			{
				Index:        0,
				Delta:        MessageDelta{},
				FinishReason: &stop,
			},
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 9,
			TotalTokens:      19,
		},
	})

	// [DONE] signal
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
}

func handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Return a single 1536-dim zero embedding
	embedding := make([]float64, 1536)
	for i := range embedding {
		embedding[i] = 0.001 * float64(i%100)
	}

	resp := EmbeddingResponse{
		Object: "list",
		Data: []EmbeddingData{
			{
				Object:    "embedding",
				Index:     0,
				Embedding: embedding,
			},
		},
		Model: req.Model,
		Usage: Usage{
			PromptTokens: 5,
			TotalTokens:  5,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	now := time.Now().Unix()
	resp := ModelList{
		Object: "list",
		Data: []Model{
			{ID: "mock-latency-test", Object: "model", Created: now, OwnedBy: "mock-provider"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok"}`)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Message: msg,
			Type:    "invalid_request_error",
			Code:    "invalid_request",
		},
	})
}

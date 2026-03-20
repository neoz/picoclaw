package sage

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"
)

// Client is an HTTP client for the Sage memory API with Ed25519 auth signing.
type Client struct {
	baseURL    string
	httpClient *http.Client
	lastTS     atomic.Int64 // monotonic timestamp to avoid replay detection
}

// NewClient creates a new Sage API client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// KnowledgeTriple is an RDF-style subject-predicate-object triple for Sage's graph.
type KnowledgeTriple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// SubmitRequest is the payload for submitting a memory to Sage.
type SubmitRequest struct {
	Content    string             `json:"content"`
	MemoryType string             `json:"memory_type"`
	Confidence float64            `json:"confidence_score"`
	DomainTag  string             `json:"domain_tag"`
	Triples    []KnowledgeTriple  `json:"knowledge_triples,omitempty"`
}

// SubmitResponse is the response from the submit endpoint.
type SubmitResponse struct {
	ID      string `json:"memory_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// MemoryItem represents a single memory returned by Sage.
type MemoryItem struct {
	ID         string  `json:"memory_id"`
	Content    string  `json:"content"`
	MemoryType string  `json:"memory_type"`
	Confidence float64 `json:"confidence_score"`
	DomainTag  string  `json:"domain_tag"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// ListResponse is the response from the list endpoint.
type ListResponse struct {
	Memories []MemoryItem `json:"memories"`
	Total    int          `json:"total"`
}

// RegisterRequest is the payload for agent registration.
type RegisterRequest struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

// Submit sends a memory to Sage.
func (c *Client) Submit(agentID string, privKey ed25519.PrivateKey, req SubmitRequest) (*SubmitResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("sage submit: marshal: %w", err)
	}

	resp, err := c.doSigned("POST", "/v1/memory/submit", body, agentID, privKey)
	if err != nil {
		return nil, fmt.Errorf("sage submit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.readError(resp)
	}

	var result SubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sage submit: decode: %w", err)
	}
	return &result, nil
}

// ListMemories retrieves memories filtered by domain tag.
func (c *Client) ListMemories(agentID string, privKey ed25519.PrivateKey, domainTags []string, limit int) (*ListResponse, error) {
	params := url.Values{}
	for _, tag := range domainTags {
		params.Add("domain_tag", tag)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	path := "/v1/memory/list?" + params.Encode()
	resp, err := c.doSigned("GET", path, nil, agentID, privKey)
	if err != nil {
		return nil, fmt.Errorf("sage list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sage list: decode: %w", err)
	}
	return &result, nil
}

// DeprecateMemory marks a memory as deprecated using the dashboard DELETE endpoint
// which directly soft-deletes the memory (sets status="deprecated").
func (c *Client) DeprecateMemory(agentID string, privKey ed25519.PrivateKey, memoryID string) error {
	path := "/v1/dashboard/memory/" + memoryID
	resp, err := c.doSigned("DELETE", path, nil, agentID, privKey)
	if err != nil {
		return fmt.Errorf("sage deprecate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// SetTags replaces all tags on a memory via the dashboard API.
func (c *Client) SetTags(agentID string, privKey ed25519.PrivateKey, memoryID string, tags []string) error {
	body, err := json.Marshal(struct {
		Tags []string `json:"tags"`
	}{Tags: tags})
	if err != nil {
		return fmt.Errorf("sage set tags: marshal: %w", err)
	}

	path := "/v1/dashboard/memory/" + memoryID + "/tags"
	resp, err := c.doSigned("PUT", path, body, agentID, privKey)
	if err != nil {
		return fmt.Errorf("sage set tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// Register registers an agent with its public key.
func (c *Client) Register(agentID string, privKey ed25519.PrivateKey, req RegisterRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("sage register: marshal: %w", err)
	}

	resp, err := c.doSigned("POST", "/v1/agent/register", body, agentID, privKey)
	if err != nil {
		return fmt.Errorf("sage register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return c.readError(resp)
	}
	return nil
}

// Health checks if the Sage server is reachable.
func (c *Client) Health() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("sage health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sage health: status %d", resp.StatusCode)
	}
	return nil
}

// doSigned performs an HTTP request with Ed25519 signature authentication.
// Signature covers: SHA256(method + " " + path + "\n" + body) || BigEndian(timestamp)
func (c *Client) doSigned(method, path string, body []byte, agentID string, privKey ed25519.PrivateKey) (*http.Response, error) {
	// Use monotonically increasing timestamp to avoid Sage replay detection
	// when multiple requests happen within the same second.
	var ts int64
	for {
		last := c.lastTS.Load()
		ts = time.Now().Unix()
		if ts <= last {
			ts = last + 1
		}
		if c.lastTS.CompareAndSwap(last, ts) {
			break
		}
	}

	// Build signing payload
	message := method + " " + path + "\n"
	if len(body) > 0 {
		message += string(body)
	}
	hash := sha256.Sum256([]byte(message))

	var tsBytes [8]byte
	binary.BigEndian.PutUint64(tsBytes[:], uint64(ts))

	payload := append(hash[:], tsBytes[:]...)
	sig := ed25519.Sign(privKey, payload)

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))
	req.Header.Set("X-Timestamp", strconv.FormatInt(ts, 10))

	return c.httpClient.Do(req)
}

func (c *Client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("sage API error %d: %s", resp.StatusCode, string(body))
}

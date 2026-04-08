package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dimiro1/lunar/internal/store"
)

type blobHandlersDBStub struct {
	store.DB

	getBlobFunc     func(context.Context, string, string) (store.Blob, error)
	createBlobFunc  func(context.Context, store.Blob) error
	deleteBlobFunc  func(context.Context, string, string) error
	updateBlobFunc  func(context.Context, store.Blob) error
	listBlobsFunc   func(context.Context, string) ([]store.Blob, error)
	getFunctionFunc func(context.Context, string) (store.Function, error)
}

func (s *blobHandlersDBStub) GetBlob(ctx context.Context, functionID, blobID string) (store.Blob, error) {
	if s.getBlobFunc != nil {
		return s.getBlobFunc(ctx, functionID, blobID)
	}
	return store.Blob{}, nil
}

func (s *blobHandlersDBStub) CreateBlob(ctx context.Context, blob store.Blob) error {
	if s.createBlobFunc != nil {
		return s.createBlobFunc(ctx, blob)
	}
	return nil
}

func (s *blobHandlersDBStub) DeleteBlob(ctx context.Context, functionID, blobID string) error {
	if s.deleteBlobFunc != nil {
		return s.deleteBlobFunc(ctx, functionID, blobID)
	}
	return nil
}

func (s *blobHandlersDBStub) UpdateBlob(ctx context.Context, blob store.Blob) error {
	if s.updateBlobFunc != nil {
		return s.updateBlobFunc(ctx, blob)
	}
	return nil
}

func (s *blobHandlersDBStub) ListBlobs(ctx context.Context, functionID string) ([]store.Blob, error) {
	if s.listBlobsFunc != nil {
		return s.listBlobsFunc(ctx, functionID)
	}
	return nil, nil
}

func (s *blobHandlersDBStub) GetFunction(ctx context.Context, functionID string) (store.Function, error) {
	if s.getFunctionFunc != nil {
		return s.getFunctionFunc(ctx, functionID)
	}
	return store.Function{}, nil
}

func TestGetBlobHandler_NotFound(t *testing.T) {
	db := &blobHandlersDBStub{
		getBlobFunc: func(_ context.Context, functionID, blobID string) (store.Blob, error) {
			if functionID != "fn1" || blobID != "b1" {
				t.Fatalf("unexpected ids: functionID=%q blobID=%q", functionID, blobID)
			}
			return store.Blob{}, store.ErrBlobNotFound
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/functions/fn1/blobs/b1", nil)
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	GetBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Blob not found") {
		t.Fatalf("expected not found message, got %q", rr.Body.String())
	}
}

func TestGetBlobHandler_Success(t *testing.T) {
	db := &blobHandlersDBStub{
		getBlobFunc: func(_ context.Context, functionID, blobID string) (store.Blob, error) {
			return store.Blob{
				ID:         blobID,
				FunctionID: functionID,
				Name:       "asset",
				MIMEType:   "text/plain",
				Content:    []byte("hello"),
				IsPublic:   true,
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/functions/fn1/blobs/b1", nil)
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	GetBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"id":"b1"`) || !strings.Contains(body, `"mime_type":"text/plain"`) {
		t.Fatalf("unexpected response body: %s", body)
	}
}

func TestCreateBlobHandler_InvalidBody(t *testing.T) {
	db := &blobHandlersDBStub{}

	req := httptest.NewRequest(http.MethodPost, "/functions/fn1/blobs/b1", bytes.NewBufferString(`{`))
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	CreateBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid request body") {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}
}

func TestCreateBlobHandler_InvalidBase64(t *testing.T) {
	db := &blobHandlersDBStub{}

	body := `{"name":"n","mimeType":"text/plain","content":"%%%","isPublic":true,"isGlobal":false}`
	req := httptest.NewRequest(http.MethodPost, "/functions/fn1/blobs/b1", bytes.NewBufferString(body))
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	CreateBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid base64 encoding of content") {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}
}

func TestCreateBlobHandler_SetsGlobalFunctionIDAndDecodesContent(t *testing.T) {
	var created store.Blob
	db := &blobHandlersDBStub{
		createBlobFunc: func(_ context.Context, blob store.Blob) error {
			created = blob
			return nil
		},
	}

	payload := base64.StdEncoding.EncodeToString([]byte("hello"))
	body := `{"name":"asset","mimeType":"text/plain","content":"` + payload + `","isPublic":true,"isGlobal":true}`
	req := httptest.NewRequest(http.MethodPost, "/functions/fn1/blobs/b1", bytes.NewBufferString(body))
	req.SetPathValue("function_id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	CreateBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if created.ID != "b1" {
		t.Fatalf("expected blob id b1, got %q", created.ID)
	}
	if created.FunctionID != "" {
		t.Fatalf("expected global blob FunctionID empty, got %q", created.FunctionID)
	}
	if string(created.Content) != "hello" {
		t.Fatalf("expected decoded content 'hello', got %q", string(created.Content))
	}
}

func TestDeleteBlobHandler_NotFound(t *testing.T) {
	db := &blobHandlersDBStub{
		deleteBlobFunc: func(_ context.Context, functionID, blobID string) error {
			return store.ErrBlobNotFound
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/functions/fn1/blobs/b1", nil)
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	DeleteBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestDeleteBlobHandler_Success(t *testing.T) {
	db := &blobHandlersDBStub{
		deleteBlobFunc: func(_ context.Context, functionID, blobID string) error {
			if functionID != "fn1" || blobID != "b1" {
				t.Fatalf("unexpected ids: %s/%s", functionID, blobID)
			}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/functions/fn1/blobs/b1", nil)
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	DeleteBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestUpdateBlobHandler_InvalidBase64(t *testing.T) {
	db := &blobHandlersDBStub{
		getBlobFunc: func(_ context.Context, _, _ string) (store.Blob, error) {
			return store.Blob{ID: "b1", FunctionID: "fn1"}, nil
		},
	}

	body := `{"content":"***"}`
	req := httptest.NewRequest(http.MethodPatch, "/functions/fn1/blobs/b1", bytes.NewBufferString(body))
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	UpdateBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid base64 encoding of content") {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}
}

func TestUpdateBlobHandler_NotFoundOnGet(t *testing.T) {
	db := &blobHandlersDBStub{
		getBlobFunc: func(_ context.Context, _, _ string) (store.Blob, error) {
			return store.Blob{}, store.ErrBlobNotFound
		},
	}

	body := `{"name":"new-name"}`
	req := httptest.NewRequest(http.MethodPatch, "/functions/fn1/blobs/b1", bytes.NewBufferString(body))
	req.SetPathValue("id", "fn1")
	req.SetPathValue("blobId", "b1")
	rr := httptest.NewRecorder()

	UpdateBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestListBlobsHandler_FunctionNotFound(t *testing.T) {
	db := &blobHandlersDBStub{
		getFunctionFunc: func(_ context.Context, _ string) (store.Function, error) {
			return store.Function{}, errors.New("missing")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/functions/fn1/blobs", nil)
	req.SetPathValue("id", "fn1")
	rr := httptest.NewRecorder()

	ListBlobsHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Function not found") {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}
}

func TestServeBlobHandler_PublicBlobServed(t *testing.T) {
	db := &blobHandlersDBStub{
		getBlobFunc: func(_ context.Context, functionID, blobID string) (store.Blob, error) {
			if functionID != "" {
				t.Fatalf("ServeBlobHandler should query global blob with empty functionID, got %q", functionID)
			}
			return store.Blob{
				ID:       blobID,
				MIMEType: "image/png",
				Content:  []byte{0x01, 0x02, 0x03},
				IsPublic: true,
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/b/public-blob", nil)
	req.SetPathValue("id", "public-blob")
	rr := httptest.NewRecorder()

	ServeBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png content-type, got %q", ct)
	}
	if !bytes.Equal(rr.Body.Bytes(), []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("unexpected body bytes: %v", rr.Body.Bytes())
	}
}

func TestServeBlobHandler_PrivateBlobReturnsNotFound(t *testing.T) {
	db := &blobHandlersDBStub{
		getBlobFunc: func(_ context.Context, _, _ string) (store.Blob, error) {
			return store.Blob{
				ID:       "private",
				IsPublic: false,
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/b/private", nil)
	req.SetPathValue("id", "private")
	rr := httptest.NewRecorder()

	ServeBlobHandler(db).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Blob not found") {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}
}

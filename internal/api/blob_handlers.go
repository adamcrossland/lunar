package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dimiro1/lunar/internal/store"
)

// GetBlobHandler returns a handler for getting a blob
func GetBlobHandler(database store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		functionID := r.PathValue("id")
		blobID := r.PathValue("blobId")

		blob, err := database.GetBlob(r.Context(), functionID, blobID)
		if err != nil {
			if errors.Is(err, store.ErrBlobNotFound) {
				writeError(w, http.StatusNotFound, "Blob not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to get blob")
			return
		}

		writeJSON(w, http.StatusOK, blob)
	}
}

// CreateBlobHandler returns a handler for creating a blob
func CreateBlobHandler(database store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		functionID := r.PathValue("id")
		blobId := r.PathValue("blobId")
		fmt.Printf("Using blobId of '%s'\n", blobId)

		var req CreateBlobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate request
		if err := ValidateCreateBlobRequest(&req, functionID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		var blob store.Blob
		blob.ID = blobId
		blob.FunctionID = functionID
		if req.IsGlobal {
			blob.FunctionID = "" // Global blobs have empty function ID
		}
		blob.Name = req.Name
		content, err := base64.StdEncoding.DecodeString(req.Content)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid base64 encoding of content")
			return
		}
		blob.MIMEType = req.MIMEType
		blob.Content = []byte(content)
		blob.IsPublic = req.IsPublic
		blob.CreatedAt = time.Now().UTC().Unix()
		blob.UpdatedAt = time.Now().UTC().Unix()

		err = database.CreateBlob(r.Context(), blob)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create blob: %v", err))
			return
		}

		writeJSON(w, http.StatusCreated, blob)
	}
}

// DeleteBlobHandler returns a handler for deleting a blob
func DeleteBlobHandler(database store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		functionID := r.PathValue("id")
		blobID := r.PathValue("blobId")

		err := database.DeleteBlob(r.Context(), functionID, blobID)
		if err != nil {
			if errors.Is(err, store.ErrBlobNotFound) {
				writeError(w, http.StatusNotFound, "Blob not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to delete blob")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// UpdateBlobHandler returns a handler for updating a blob
func UpdateBlobHandler(database store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		functionID := r.PathValue("id")
		blobID := r.PathValue("blobId")

		var req UpdateBlobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate request
		if err := ValidateUpdateBlobRequest(&req, functionID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		blob, err := database.GetBlob(r.Context(), functionID, blobID)
		if err != nil {
			if errors.Is(err, store.ErrBlobNotFound) {
				writeError(w, http.StatusNotFound, "Blob not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to get blob")
			return
		}

		if req.Name != nil {
			blob.Name = *req.Name
		}
		if req.MIMEType != nil {
			blob.MIMEType = *req.MIMEType
		}
		if req.Content != nil {
			content, err := base64.StdEncoding.DecodeString(*req.Content)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid base64 encoding of content")
				return
			}
			blob.Content = []byte(content)
		}
		if req.IsPublic != nil {
			blob.IsPublic = *req.IsPublic
		}
		if req.IsGlobal != nil {
			if *req.IsGlobal {
				blob.FunctionID = "" // Global blobs have empty function ID
			} else {
				blob.FunctionID = functionID
			}
		}

		err = database.UpdateBlob(r.Context(), blob)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update blob")
			return
		}

		writeJSON(w, http.StatusOK, blob)
	}
}

// ListBlobsHandler returns a handler for listing blobs
func ListBlobsHandler(database store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		functionID := r.PathValue("id")

		// Verify function exists
		if _, err := database.GetFunction(r.Context(), functionID); err != nil {
			writeError(w, http.StatusNotFound, "Function not found")
			return
		}

		blobs, err := database.ListBlobs(r.Context(), functionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to list blobs")
			return
		}

		writeJSON(w, http.StatusOK, blobs)
	}
}

func ServeBlobHandler(database store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		blobID := r.PathValue("id")

		blob, err := database.GetBlob(r.Context(), "", blobID)
		if err != nil {
			if errors.Is(err, store.ErrBlobNotFound) {
				writeError(w, http.StatusNotFound, "Blob not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to get blob")
			return
		}

		if !blob.IsPublic {
			// Don't reveal the existence of the blob if it's not public
			writeError(w, http.StatusNotFound, "Blob not found")
			return
		}

		w.Header().Set("Content-Type", blob.MIMEType)
		w.WriteHeader(http.StatusOK)
		w.Write(blob.Content)
	}
}

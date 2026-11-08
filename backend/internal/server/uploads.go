package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxImageBytes = 5 << 20

var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+(128<<10))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "image_too_large", "image files must be 5 MB or smaller")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_upload", "send one image in the image form field")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image_required", "choose an image to upload")
		return
	}
	defer file.Close()

	contents, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		s.internalError(w, "read uploaded image", err)
		return
	}
	if len(contents) == 0 {
		writeError(w, http.StatusBadRequest, "empty_image", "the selected image is empty")
		return
	}
	if len(contents) > maxImageBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "image_too_large", "image files must be 5 MB or smaller")
		return
	}

	contentType := http.DetectContentType(contents)
	extension, ok := imageExtensions[strings.Split(contentType, ";")[0]]
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_image_type", "upload a JPEG, PNG, or WebP image")
		return
	}
	var randomName [16]byte
	if _, err := rand.Read(randomName[:]); err != nil {
		s.internalError(w, "generate uploaded image name", err)
		return
	}
	filename := hex.EncodeToString(randomName[:]) + extension
	path := filepath.Join(s.config.UploadDir, filename)
	storedFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		s.internalError(w, "create uploaded image", err)
		return
	}
	if _, err := storedFile.Write(contents); err != nil {
		_ = storedFile.Close()
		_ = os.Remove(path)
		s.internalError(w, "save uploaded image", err)
		return
	}
	if err := storedFile.Close(); err != nil {
		_ = os.Remove(path)
		s.internalError(w, "close uploaded image", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"image_url": "/uploads/" + filename})
}

func (s *Server) getUploadedImage(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if !uploadedImageName.MatchString(filename) || filepath.Base(filename) != filename {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.config.UploadDir, filename)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.internalError(w, "open uploaded image", err)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		s.internalError(w, "inspect uploaded image", err)
		return
	}
	if !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	contentType := "image/jpeg"
	switch filepath.Ext(filename) {
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

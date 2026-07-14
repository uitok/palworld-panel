package api

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/mods"
)

func (s Server) inspectModImport(c *gin.Context) {
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	var inspection mods.ImportInspection
	var err error
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if s.cfg.MaxUploadBytes > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, multipartRequestLimit(s.cfg.MaxUploadBytes))
		}
		file, header, formErr := c.Request.FormFile("file")
		if formErr != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(formErr, &maxBytesError) {
				fail(c, http.StatusRequestEntityTooLarge, "archive_too_large", "uploaded mod exceeds PALPANEL_MAX_UPLOAD_MB")
				return
			}
			fail(c, http.StatusBadRequest, "upload_missing_file", "a local ZIP file is required")
			return
		}
		defer file.Close()
		if s.cfg.MaxUploadBytes > 0 && header.Size > s.cfg.MaxUploadBytes {
			fail(c, http.StatusRequestEntityTooLarge, "archive_too_large", "uploaded mod exceeds PALPANEL_MAX_UPLOAD_MB")
			return
		}
		inspection, err = s.mods.InspectUpload(c.Request.Context(), file, header.Filename)
	} else {
		var request struct {
			Source string `json:"source" binding:"required"`
		}
		if bindErr := c.ShouldBindJSON(&request); bindErr != nil {
			fail(c, http.StatusBadRequest, "invalid_json", "source is required")
			return
		}
		inspection, err = s.mods.InspectSource(c.Request.Context(), request.Source)
	}
	if err != nil {
		failModImport(c, err)
		return
	}
	ok(c, inspection)
}

func (s Server) selectModImportCandidate(c *gin.Context) {
	var request struct {
		CandidateID string `json:"candidate_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", "candidate_id is required")
		return
	}
	inspection, err := s.mods.SelectImportCandidate(c.Request.Context(), c.Param("id"), request.CandidateID)
	if err != nil {
		failModImport(c, err)
		return
	}
	ok(c, inspection)
}

func (s Server) startModImport(c *gin.Context) {
	var request struct {
		InspectionID string `json:"inspection_id" binding:"required"`
		CandidateID  string `json:"candidate_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", "inspection_id is required")
		return
	}
	job, err := s.mods.Import(c.Request.Context(), request.InspectionID, request.CandidateID)
	if err != nil {
		failModImport(c, err)
		return
	}
	accepted(c, job)
}

func failModImport(c *gin.Context, err error) {
	var failure mods.ImportFailure
	if !errors.As(err, &failure) {
		fail(c, http.StatusBadRequest, "mod_import_failed", err.Error())
		return
	}
	status := http.StatusBadRequest
	switch failure.Code {
	case "inspection_not_found", "candidate_not_found":
		status = http.StatusNotFound
	case "inspection_expired":
		status = http.StatusGone
	case "inspection_claimed", "inspection_unavailable":
		status = http.StatusConflict
	case "archive_too_large":
		status = http.StatusRequestEntityTooLarge
	case "download_failed", "github_release_failed", "github_release_invalid":
		status = http.StatusBadGateway
	}
	fail(c, status, failure.Code, failure.Error())
}

const multipartFormOverhead int64 = 1 << 20

func multipartRequestLimit(fileLimit int64) int64 {
	if fileLimit > math.MaxInt64-multipartFormOverhead {
		return math.MaxInt64
	}
	return fileLimit + multipartFormOverhead
}

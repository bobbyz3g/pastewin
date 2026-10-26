package main

import (
	"encoding/json/v2"
	"errors"

	"pastewin/internal/updatecheck"
)

// CheckForUpdates checks only when requested by the settings UI.
func (s *ClipboardService) CheckForUpdates() (updatecheck.Result, error) {
	var metadata struct {
		Fixed struct {
			Version string `json:"file_version"`
		} `json:"fixed"`
	}
	if err := json.Unmarshal(buildMetadata, &metadata); err != nil {
		return updatecheck.Result{}, updatecheck.ErrVersion
	}
	return updatecheck.Check(s.ctx, metadata.Fixed.Version)
}

// OpenUpdateRelease opens a validated project release in the default browser.
func (s *ClipboardService) OpenUpdateRelease(tag string) error {
	url, err := updatecheck.ReleaseURL(tag)
	if err != nil {
		return err
	}
	if err := s.app.Browser.OpenURL(url); err != nil {
		return errors.New("无法打开浏览器，请稍后重试")
	}
	return nil
}

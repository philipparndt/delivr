package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// App preview upload.
//
// Previews mirror screenshots — reserve, PUT the parts described by
// uploadOperations, commit with an MD5 — but differ in three ways that matter:
//
//   - the set is keyed by `previewType`, not `screenshotDisplayType`
//   - reservation carries a `mimeType`, and is rejected without it
//   - a preview has a poster frame, given as an HH:MM:SS:FF timecode. Left
//     unset, Apple picks one, and the opening frames of a preview are usually
//     the least representative of it.
//
// Apple also transcodes previews after upload, so a file that reserves and
// commits cleanly can still be rejected minutes later. Everything checkable is
// checked before the bytes go out.

type createPreview struct {
	Data createPreviewData `json:"data"`
}

type createPreviewData struct {
	Type          string               `json:"type"`
	Attributes    createPreviewAttrs   `json:"attributes"`
	Relationships previewRelationships `json:"relationships"`
}

type createPreviewAttrs struct {
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
	MimeType string `json:"mimeType,omitempty"`
}

type previewRelationships struct {
	AppPreviewSet relationship `json:"appPreviewSet"`
}

type commitPreview struct {
	Data commitPreviewData `json:"data"`
}

type commitPreviewData struct {
	Type       string             `json:"type"`
	ID         string             `json:"id"`
	Attributes commitPreviewAttrs `json:"attributes"`
}

type commitPreviewAttrs struct {
	Uploaded             bool   `json:"uploaded"`
	SourceFileChecksum   string `json:"sourceFileChecksum"`
	PreviewFrameTimeCode string `json:"previewFrameTimeCode,omitempty"`
}

type createPreviewSet struct {
	Data createPreviewSetData `json:"data"`
}

type createPreviewSetData struct {
	Type          string                  `json:"type"`
	Attributes    createPreviewSetAttrs   `json:"attributes"`
	Relationships previewSetRelationships `json:"relationships"`
}

type createPreviewSetAttrs struct {
	PreviewType string `json:"previewType"`
}

type previewSetRelationships struct {
	AppStoreVersionLocalization relationship `json:"appStoreVersionLocalization"`
}

// UploadPreviewSet replaces the previews for one localization and preview type.
func (c *Client) UploadPreviewSet(localizationID, previewType string,
	files []string, posterFrame float64, onFileUploaded func()) error {

	setID, err := c.findPreviewSet(localizationID, previewType)
	if err != nil {
		return err
	}
	if setID == "" {
		setID, err = c.createPreviewSet(localizationID, previewType)
		if err != nil {
			return fmt.Errorf("create preview set %s: %w", previewType, err)
		}
	} else if err := c.deletePreviewsInSet(setID); err != nil {
		return fmt.Errorf("clear preview set %s: %w", previewType, err)
	}

	for _, f := range files {
		if err := c.uploadPreview(setID, f, posterFrame); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		if onFileUploaded != nil {
			onFileUploaded()
		}
	}
	return nil
}

func (c *Client) findPreviewSet(localizationID, previewType string) (string, error) {
	data, err := c.get(fmt.Sprintf(
		"/appStoreVersionLocalizations/%s/appPreviewSets?fields[appPreviewSets]=previewType",
		localizationID))
	if err != nil {
		return "", err
	}

	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	for _, item := range resp.Data {
		var attrs struct {
			PreviewType string `json:"previewType"`
		}
		if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
			continue
		}
		if attrs.PreviewType == previewType {
			return item.ID, nil
		}
	}
	return "", nil
}

func (c *Client) createPreviewSet(localizationID, previewType string) (string, error) {
	data, err := c.post("/appPreviewSets", createPreviewSet{
		Data: createPreviewSetData{
			Type:       "appPreviewSets",
			Attributes: createPreviewSetAttrs{PreviewType: previewType},
			Relationships: previewSetRelationships{
				AppStoreVersionLocalization: relationship{
					Data: relationshipData{
						Type: "appStoreVersionLocalizations",
						ID:   localizationID,
					},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	var resp singleResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}

func (c *Client) deletePreviewsInSet(setID string) error {
	data, err := c.get(fmt.Sprintf(
		"/appPreviewSets/%s/appPreviews?fields[appPreviews]=fileName", setID))
	if err != nil {
		return err
	}
	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	for _, item := range resp.Data {
		if err := c.del(fmt.Sprintf("/appPreviews/%s", item.ID)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) uploadPreview(setID, filePath string, posterFrame float64) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	// 1. Reserve. mimeType is required here, unlike for screenshots.
	reserveData, err := c.post("/appPreviews", createPreview{
		Data: createPreviewData{
			Type: "appPreviews",
			Attributes: createPreviewAttrs{
				FileName: filepath.Base(filePath),
				FileSize: info.Size(),
				MimeType: mimeTypeForVideo(filePath),
			},
			Relationships: previewRelationships{
				AppPreviewSet: relationship{
					Data: relationshipData{Type: "appPreviewSets", ID: setID},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("reserve: %w", err)
	}

	var reserveResp singleResponse
	if err := json.Unmarshal(reserveData, &reserveResp); err != nil {
		return err
	}

	// Same shape as a screenshot reservation: a list of uploadOperations.
	var attrs screenshotAttrs
	if err := json.Unmarshal(reserveResp.Data.Attributes, &attrs); err != nil {
		return err
	}

	// 2. Upload the parts. Previews run to tens of megabytes, so unlike a
	//    screenshot this is genuinely chunked.
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	for _, op := range attrs.UploadOperations {
		end := op.Offset + op.Length
		if end > len(fileData) {
			end = len(fileData)
		}
		headers := make(map[string]string)
		for _, h := range op.RequestHeaders {
			headers[h.Name] = h.Value
		}
		if err := c.uploadRaw(op.Method, op.URL, fileData[op.Offset:end], headers); err != nil {
			return fmt.Errorf("upload part at offset %d: %w", op.Offset, err)
		}
	}

	// 3. Commit, naming the poster frame.
	checksum, err := fileMD5(filePath)
	if err != nil {
		return err
	}
	return c.patch(fmt.Sprintf("/appPreviews/%s", reserveResp.Data.ID), commitPreview{
		Data: commitPreviewData{
			Type: "appPreviews",
			ID:   reserveResp.Data.ID,
			Attributes: commitPreviewAttrs{
				Uploaded:             true,
				SourceFileChecksum:   checksum,
				PreviewFrameTimeCode: timeCode(posterFrame),
			},
		},
	})
}

func mimeTypeForVideo(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mov":
		return "video/quicktime"
	default:
		return "video/mp4"
	}
}

// timeCode renders seconds as the HH:MM:SS:FF Apple expects. Frames are at 30,
// which is what delivr's own encoder produces.
func timeCode(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	total := int(seconds)
	frames := int((seconds - float64(total)) * 30)
	return fmt.Sprintf("%02d:%02d:%02d:%02d",
		total/3600, (total%3600)/60, total%60, frames)
}

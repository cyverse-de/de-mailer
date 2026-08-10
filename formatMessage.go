package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	html "html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	text "text/template"
	"time"

	"github.com/mitchellh/mapstructure"
)

// email request received
type EmailRequest struct {
	FromAddr    string
	To          string
	Cc          []string
	Bcc         []string
	Template    string
	Subject     string
	Attachments []struct {
		Filename string
		Data     string // Base64-encoded file data
	}
	Values json.RawMessage
}

type Templater interface {
	Execute(io.Writer, any) error
}

// VICERequestCompleteDetails contains the request detail fields that we need to extract when a VICE access request is
// marked as complete.
type VICERequestCompleteDetails struct {
	ConcurrentJobs int64  `mapstructure:"concurrent_jobs"`
	UseCase        string `mapstructure:"intended_use"`
}

// ToolRequestDetails contains the request detail fields that we need to extract for tool requests.
type ToolRequestDetails struct {
	Description   string `mapstructure:"description"`
	Documentation string `mapstructure:"documentation_url"`
	Source        string `mapstructure:"source_url"`
	Name          string `mapstructure:"name"`
	TestData      string `mapstructure:"test_data_path"`
	SubmittedBy   string `mapstructure:"submitted_by"`
}

// RequestSubmittedDetails contains the request detail fields that we need to extract for request submissions.
type RequestSubmittedDetails struct {
	Name           string `mapstructure:"name"`
	Email          string `mapstructure:"email"`
	UseCase        string `mapstructure:"intended_use"`
	ConcurrentJobs int64  `mapstructure:"concurrent_jobs"`
}

// ExtractDetails extracts fields from a nested object in the payload.
func ExtractDetails(payload map[string]any, dest any, fieldNames ...string) error {
	for _, fieldName := range fieldNames {
		source, ok := payload[fieldName]
		if ok && source != nil {
			return mapstructure.Decode(source, dest)
		}
	}

	return fmt.Errorf("required payload field not found: %s", fieldNames)
}

// format email message using templates
func FormatMessage(ctx context.Context, emailReq EmailRequest, payload map[string](any), deSettings DESettings) (bytes.Buffer, bool, error) {
	log := log.WithContext(ctx)

	log.Infof("Received formatting request with template %s", emailReq.Template)
	var template_output bytes.Buffer

	payload["DELink"] = deSettings.base
	payload["DEDataLink"] = deSettings.base + deSettings.data
	payload["DETeamsLink"] = deSettings.base + deSettings.teams
	payload["DEAdminDoiRequestLink"] = deSettings.base + deSettings.admin + deSettings.doi
	payload["DEToolsLink"] = deSettings.base + deSettings.tools
	payload["DECollectionsLink"] = deSettings.base + deSettings.collections
	payload["DEAppsLink"] = deSettings.base + deSettings.apps
	payload["DEAnalysesLink"] = deSettings.base + deSettings.analyses
	payload["DEPublicationRequestsLink"] = deSettings.base + deSettings.admin + deSettings.apps
	payload["DEPidRequestLink"] = deSettings.base + deSettings.admin + deSettings.doi

	var isHtml = false
	var tmpl Templater
	var err error

	// Only a definitively absent template is the caller's fault; any other stat failure
	// (permissions, bad mount) is a server-side 500 so it can't be mistaken for bad input.
	htmlPath := "./templates/html/" + emailReq.Template + ".tmpl"
	textPath := "./templates/text/" + emailReq.Template + ".tmpl"
	if _, statErr := os.Stat(htmlPath); statErr == nil {
		isHtml = true
		tmpl, err = html.ParseFiles(htmlPath, "./templates/html/header.tmpl", "./templates/html/footer.tmpl")
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return template_output, isHtml, statErr
	} else if _, statErr := os.Stat(textPath); statErr == nil {
		tmpl, err = text.ParseFiles(textPath)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return template_output, isHtml, statErr
	} else {
		return template_output, isHtml, NewHTTPError(http.StatusBadRequest, "unknown template: %q", emailReq.Template)
	}
	if err != nil {
		log.Error(err)
		return template_output, isHtml, err
	}

	switch emailReq.Template {
	case "analysis_status_change", "analysis_periodic_notification":
		var startDateText, resultFolderPath, analysisId string

		// Format the analysis start date.
		err = ExtractDetails(payload, &startDateText, "startdate")
		if err != nil {
			log.Errorf("unable to extract the analysis start date: %s", err)
			startDateText = ""
		}
		mill_sec, parse_err := strconv.ParseInt(startDateText, 10, 64)
		if parse_err != nil {
			log.Error(parse_err)
		}
		start_date := time.Unix(0, mill_sec*int64(time.Millisecond))
		payload["startdate"] = start_date

		// Format the link to the analysis result folder.
		err = ExtractDetails(payload, &resultFolderPath, "analysisresultsfolder", "result_folder_path")
		if err != nil {
			log.Errorf("unable to extract the analysis result folder path: %s", err)
		}
		payload["DEOutputFolderLink"] = deSettings.base + deSettings.data + resultFolderPath

		// Format the link to the analysis details page
		err = ExtractDetails(payload, &analysisId, "analysisid")
		if err != nil {
			log.Errorf("unable to extract the analysis ID: %s", err)
		}
		payload["DEAnalysisDetailsLink"] = deSettings.base + deSettings.analyses + "/" + analysisId

	case "added_to_team":
		var teamName string
		err = ExtractDetails(payload, &teamName, "team_name")
		if err != nil {
			return template_output, isHtml, NewHTTPError(http.StatusBadRequest, "unable to extract the team name: %s", err)
		}
		payload["DETeamsLink"] = deSettings.base + deSettings.teams + "/" + teamName

	case "request_complete", "request_rejected":
		// A missing request_type used to panic here, so no legitimate payload omits it;
		// silently rendering as non-VICE would send an email without the VICE details.
		requestType, ok := payload["request_type"].(string)
		if !ok {
			return template_output, isHtml, NewHTTPError(http.StatusBadRequest, "missing or non-string request_type in %s payload", emailReq.Template)
		}
		if requestType == "vice" {
			var viceRequestDetails VICERequestCompleteDetails
			err := ExtractDetails(payload, &viceRequestDetails, "request_details")
			if err != nil {
				return template_output, isHtml, NewHTTPError(http.StatusBadRequest, "unable to extract the VICE request details: %s", err)
			}
			payload["ConcurrentJobs"] = viceRequestDetails.ConcurrentJobs
			payload["UseCase"] = viceRequestDetails.UseCase
			payload["DEAppsLink"] = deSettings.base + deSettings.apps + "?selectedFilter={\"value\":\"Interactive\",\"display\":\"VICE\"}&selectedCategory={\"name\":\"Browse All Apps\",\"id\":\"pppppppp-pppp-pppp-pppp-pppppppppppp\"}"
		}
	case "tool_request":
		var reqDetails ToolRequestDetails
		err := ExtractDetails(payload, &reqDetails, "toolrequestdetails")
		if err != nil {
			return template_output, isHtml, NewHTTPError(http.StatusBadRequest, "unable to extract the tool request details: %s", err)
		}
		payload["user"] = "Admin"
		payload["Description"] = reqDetails.Description
		payload["Documentation"] = reqDetails.Documentation
		payload["Source"] = reqDetails.Source
		payload["Name"] = reqDetails.Name
		payload["TestData"] = reqDetails.TestData
		payload["SubmittedBy"] = reqDetails.SubmittedBy
		payload["DEToolRequestLink"] = deSettings.base + deSettings.admin + deSettings.tools

	case "request_submitted":
		var reqDetails RequestSubmittedDetails
		err := ExtractDetails(payload, &reqDetails, "request_details")
		if err != nil {
			return template_output, isHtml, NewHTTPError(http.StatusBadRequest, "unable to extract the request details: %s", err)
		}
		payload["Name"] = reqDetails.Name
		payload["Email"] = reqDetails.Email
		payload["UseCase"] = reqDetails.UseCase
		payload["ConcurrentJobs"] = reqDetails.ConcurrentJobs
		payload["user"] = "Admin"
	}

	tmpl_err := tmpl.Execute(&template_output, payload)
	if tmpl_err != nil {
		log.Error(tmpl_err)
	}
	return template_output, isHtml, tmpl_err

}

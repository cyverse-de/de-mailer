package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"strconv"
	"time"

	"github.com/cyverse-de/logcabin"
)

// message in the email request
type message struct {
	Text string
}

// email request received
type EmailRequest struct {
	FromAddr string
	To       string
	Template string
	Subject  string
	Values   json.RawMessage
}

//format email message using templates
func FormatMessage(emailReq EmailRequest, payload map[string](interface{}), deSettings DESettings) (bytes.Buffer, error) {
	logcabin.Info.Println("Received formatting request with template " + emailReq.Template)
	var template_output bytes.Buffer

	payload["DELink"] = deSettings.base
	payload["DETeamsLink"] = deSettings.base + deSettings.teams
	payload["DEAdminDoiRequestLink"] = deSettings.base + deSettings.admin + deSettings.doi
	payload["DEToolsLink"] = deSettings.base + deSettings.tools
	payload["DECommunitiesLink"] = deSettings.base + deSettings.communities
	payload["DEAppsLink"] = deSettings.base + deSettings.apps
	payload["DEPublicationRequestsLink"] = deSettings.base + deSettings.admin + deSettings.apps
	payload["DEPidRequestLink"] = deSettings.base + deSettings.admin + deSettings.doi

	tmpl, err := template.ParseFiles("./templates/"+emailReq.Template+".tmpl", "./templates/header.tmpl", "./templates/footer.tmpl")
	if err != nil {
		logcabin.Error.Println(err)
		return template_output, err
	}

	switch emailReq.Template {
	case "analysis_status_change":
		mill_sec, parse_err := strconv.ParseInt(payload["startdate"].(string), 10, 64)
		if parse_err != nil {
			logcabin.Error.Println(parse_err)
		}
		start_date := time.Unix(0, mill_sec*int64(time.Millisecond))
		payload["DEOutputFolderLink"] = deSettings.base + deSettings.data + payload["analysisresultsfolder"].(string)
		payload["startdate"] = start_date

	case "added_to_team":
		payload["DETeamsLink"] = deSettings.base + deSettings.teams + "/" + payload["team_name"].(string)

	case "request_complete":
		if payload["request_type"].(string) == "vice" {
			reqDetails := payload["request_details"].(map[string]interface{})
			payload["DEAppsLink"] = deSettings.base + deSettings.apps + "?selectedFilter={\"value\":\"Interactive\",\"display\":\"VICE\"}&selectedCategory={\"name\":\"Browse All Apps\",\"id\":\"pppppppp-pppp-pppp-pppp-pppppppppppp\"}"
			payload["ConcurrentJobs"] = reqDetails["concurrent_jobs"].(float64)
			payload["UseCase"] = reqDetails["intended_use"].(string)
		}
	case "tool_request":
		reqDetails := payload["toolrequestdetails"].(map[string]interface{})
		payload["Description"] = reqDetails["description"].(string)
		payload["Documentation"] = reqDetails["documentation_url"].(string)
		payload["Source"] = reqDetails["source_url"].(string)
		payload["Name"] = reqDetails["name"].(string)
		payload["TestData"] = reqDetails["test_data_path"].(string)
		payload["SubmittedBy"] = reqDetails["submitted_by"].(string)
	}

	tmpl_err := tmpl.Execute(&template_output, payload)
	if tmpl_err != nil {
		logcabin.Error.Println(tmpl_err)
	}
	return template_output, tmpl_err

}

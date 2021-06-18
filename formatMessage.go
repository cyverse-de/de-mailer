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
	User           string
	Email_Template string
	Type           string
	Subject        string
	Message        message
	Payload        json.RawMessage
}

//format email message using templates
func FormatMessage(emailReq EmailRequest, payload map[string](interface{}), deSettings DESettings) (bytes.Buffer, error) {
	logcabin.Info.Println("Received formatting request with template " + emailReq.Email_Template + " for user " + emailReq.User)
	messageType := emailReq.Type
	var template_output bytes.Buffer

	payload["user"] = emailReq.User
	payload["DELink"] = deSettings.base
	payload["DETeamsLink"] = deSettings.base + deSettings.teams
	payload["DEAdminDoiRequestLink"] = deSettings.base + deSettings.admin + deSettings.doi
	payload["DEToolsLink"] = deSettings.base + deSettings.tools
	payload["DECommunitiesLink"] = deSettings.base + deSettings.communities
	payload["DEAppsLink"] = deSettings.base + deSettings.apps
	payload["DEPublicationRequestsLink"] = deSettings.base + deSettings.admin + deSettings.apps
	payload["DEPidRequestLink"] = deSettings.base + deSettings.admin + deSettings.doi

	tmpl, err := template.ParseFiles("./templates/"+emailReq.Email_Template+".tmpl", "./templates/header.tmpl", "./templates/footer.tmpl")
	if err != nil {
		logcabin.Error.Println(err)
		return template_output, err
	}

	if messageType == "analysis" {
		mill_sec, parse_err := strconv.ParseInt(payload["startdate"].(string), 10, 64)
		if parse_err != nil {
			logcabin.Error.Println(parse_err)
		}
		start_date := time.Unix(0, mill_sec*int64(time.Millisecond))
		payload["DEOutputFolderLink"] = deSettings.base + deSettings.data + payload["analysisresultsfolder"].(string)
		payload["startdate"] = start_date
		tmpl_err := tmpl.Execute(&template_output, payload)
		if tmpl_err != nil {
			logcabin.Error.Println(tmpl_err)
		}
		return template_output, tmpl_err
	}

	tmpl_err := tmpl.Execute(&template_output, payload)
	if tmpl_err != nil {
		logcabin.Error.Println(tmpl_err)
	}
	return template_output, tmpl_err

}
